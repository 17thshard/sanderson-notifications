package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type TwitterPlugin struct {
	Token                   string
	Account                 string
	Nickname                string
	TweetMessage            string   `mapstructure:"tweetMessage"`
	RetweetMessage          string   `mapstructure:"retweetMessage"`
	ExcludedRetweetAccounts []string `mapstructure:"excludeRetweetsOf"`
	retweetExclusions       map[string]bool
}

func (plugin *TwitterPlugin) Name() string {
	return "twitter"
}

func (plugin *TwitterPlugin) Validate() error {
	if len(plugin.Token) == 0 {
		return fmt.Errorf("token for Twitter must not be empty")
	}

	plugin.retweetExclusions = make(map[string]bool)
	for _, account := range plugin.ExcludedRetweetAccounts {
		plugin.retweetExclusions[account] = true
	}

	return nil
}

func (plugin *TwitterPlugin) OffsetPrototype() interface{} {
	return ""
}

type Tweet struct {
	Id              uint64
	User            TweetUser
	RetweetedStatus *Tweet  `json:"retweeted_status"`
	ReplyToUsername *string `json:"in_reply_to_screen_name"`
}

type TweetUser struct {
	Name    string
	Account string `json:"screen_name"`
}

func (plugin *TwitterPlugin) Check(offset interface{}, context PluginContext) (interface{}, error) {
	if offset == nil {
		return nil, fmt.Errorf("latest Tweet ID must be specified as offset for start")
	}

	context.Info.Println("Checking for new tweets...")

	lastTweet := offset.(string)
	if len(lastTweet) == 0 {
		return nil, fmt.Errorf("latest Tweet ID must be specified as offset for start")
	}

	tweets, err := plugin.retrieveTweetsSince(lastTweet, context)
	if err != nil {
		return lastTweet, err
	}

	if len(tweets) == 0 {
		context.Info.Println("No tweets to report.")
		return lastTweet, nil
	}

	context.Info.Printf("Reporting %d tweets...\n", len(tweets))

	if len(plugin.Nickname) == 0 && (len(plugin.TweetMessage) == 0 || len(plugin.RetweetMessage) == 0) {
		plugin.Nickname = tweets[0].User.Name
		context.Info.Printf(
			"No nickname or specific messages were provided for account '%s', using name '%s' as fallback nickname",
			plugin.Account,
			plugin.Nickname,
		)
	}

	for i := len(tweets) - 1; i >= 0; i-- {
		tweet := tweets[i]
		if tweet.RetweetedStatus != nil {
			if exclude, present := plugin.retweetExclusions[tweet.RetweetedStatus.User.Account]; present && exclude {
				context.Info.Printf(
					"Ignoring retweet %d from '%s', as the original tweet is from '%s'",
					tweet.Id,
					tweet.User.Account,
					tweet.RetweetedStatus.User.Account,
				)
				lastTweet = strconv.FormatUint(tweet.Id, 10)
				continue
			}
		}

		if tweet.ReplyToUsername != nil && *tweet.ReplyToUsername != plugin.Account {
			context.Info.Printf(
				"Ignoring reply tweet %d from '%s', as it is not in response to themself",
				tweet.Id,
				tweet.User.Account,
			)
			lastTweet = strconv.FormatUint(tweet.Id, 10)
			continue
		}

		message := fmt.Sprintf("%s tweeted", plugin.Nickname)
		if len(plugin.TweetMessage) > 0 {
			message = plugin.TweetMessage
		}
		if tweet.RetweetedStatus != nil {
			message = fmt.Sprintf("%s retweeted", plugin.Nickname)
			if len(plugin.RetweetMessage) > 0 {
				message = plugin.RetweetMessage
			}
		}

		if err = context.Discord.Send(
			fmt.Sprintf("%s https://twitter.com/%s/status/%d", message, plugin.Account, tweet.Id),
			"Twitter",
			"twitter",
			nil,
		); err != nil {
			return lastTweet, err
		}

		lastTweet = strconv.FormatUint(tweet.Id, 10)
	}

	return lastTweet, nil
}

func (plugin *TwitterPlugin) retrieveTweetsSince(lastTweet string, context PluginContext) ([]Tweet, error) {
	client := &http.Client{}

	var (
		result []Tweet
		maxId  uint64
	)
	for {
		tweets, err := plugin.tryRead(client, context, lastTweet, maxId, 1)
		if err != nil {
			return nil, fmt.Errorf("could not read tweets: %w", err)
		}

		// Max ID is inclusive, so we can't rely on receiving an empty result
		if len(tweets) == 0 || (len(tweets) == 1 && tweets[0].Id == maxId) {
			break
		}

		for _, tweet := range tweets {
			if tweet.Id != maxId {
				result = append(result, tweet)
			}
		}

		maxId = tweets[len(tweets)-1].Id
	}

	return result, nil
}

const maxRetries = 3

func (plugin *TwitterPlugin) tryRead(client *http.Client, context PluginContext, since string, max uint64, try int) ([]Tweet, error) {
	timelineUrl := fmt.Sprintf(
		"https://api.twitter.com/1.1/statuses/user_timeline.json"+
			"?screen_name=%s"+
			"&since_id=%s"+
			"&count=100"+
			"&exclude_replies=false"+
			"&include_rts=true",
		url.QueryEscape(plugin.Account),
		url.QueryEscape(since),
	)

	if max != 0 {
		timelineUrl += fmt.Sprintf("&max_id=%s", url.QueryEscape(strconv.FormatUint(max, 10)))
	}

	req, err := http.NewRequest("GET", timelineUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build tweets request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", plugin.Token))
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't request tweets: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		if try == maxRetries {
			return nil, fmt.Errorf("rate limiting still applied after %d retries", maxRetries)
		}

		if res.Header.Get("X-App-Rate-Limit-Remaining") == "0" {
			return nil, fmt.Errorf("app rate limit hit for current 24h period")
		}

		rawLimitResetTime := res.Header.Get("X-Rate-Limit-Reset")
		if rawLimitResetTime == "" {
			return nil, fmt.Errorf("no rate limit reset time found")
		}

		limitResetTimeValue, err := strconv.ParseInt(rawLimitResetTime, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("rate limit '%s' could not be parsed into number: %w", rawLimitResetTime, err)
		}

		delay := time.Unix(limitResetTimeValue, 0).Sub(time.Now())

		context.Info.Printf("Being rate late limited by Twitter, waiting for %ds\n", delay)
		time.Sleep(delay)

		return plugin.tryRead(client, context, since, max, try+1)
	}

	if res.StatusCode != http.StatusOK {
		responseBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("couldn't read response body for response '%s': %w", res.Status, err)
		}

		return nil, fmt.Errorf("received response '%s', body was: %s", res.Status, string(responseBody))
	}

	var result []Tweet
	err = json.NewDecoder(res.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse tweets: %w", err)
	}

	return result, nil
}
