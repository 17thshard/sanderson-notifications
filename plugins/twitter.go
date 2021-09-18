package plugins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
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

	if len(plugin.Nickname) == 0 && (len(plugin.TweetMessage) == 0 || len(plugin.TweetMessage) == 0) {
		return fmt.Errorf("either an account nickname or tweet and retweet messages must be given")
	}

	plugin.retweetExclusions = make(map[string]bool)
	for _, account := range plugin.ExcludedRetweetAccounts {
		plugin.retweetExclusions[account] = true
	}

	return nil
}

func (plugin *TwitterPlugin) OffsetType() reflect.Type {
	return reflect.TypeOf("")
}

type Tweet struct {
	Id              uint64
	User            TweetUser
	RetweetedStatus *Tweet `json:"retweeted_status"`
}

type TweetUser struct {
	Account string `json:"screen_name"`
}

func (plugin *TwitterPlugin) Check(offset interface{}, context PluginContext) (interface{}, error) {
	if offset == nil {
		return nil, fmt.Errorf("latest Tweet ID must be specified as offset for start")
	}

	context.Info.Println("Checking for new tweets...")

	lastTweet := offset.(string)

	tweets, err := plugin.retrieveTweetsSince(lastTweet)
	if err != nil {
		return lastTweet, err
	}

	if len(tweets) == 0 {
		context.Info.Println("No tweets to report.")
		return lastTweet, nil
	}

	context.Info.Printf("Reporting %d tweets...\n", len(tweets))

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
				continue
			}
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

		context.Discord.Send(
			fmt.Sprintf("%s https://twitter.com/%s/status/%d", message, plugin.Account, tweet.Id),
			"Twitter",
			"twitter",
			nil,
		)
	}

	return strconv.FormatUint(tweets[0].Id, 10), nil
}

func (plugin *TwitterPlugin) retrieveTweetsSince(lastTweet string) ([]Tweet, error) {
	client := &http.Client{}
	timelineUrl := fmt.Sprintf(
		"https://api.twitter.com/1.1/statuses/user_timeline.json"+
			"?screen_name=%s"+
			"&since_id=%s"+
			"&exclude_replies=true"+
			"&include_rts=true"+
			"&count=100",
		url.QueryEscape(plugin.Account),
		url.QueryEscape(lastTweet),
	)

	req, err := http.NewRequest("GET", timelineUrl, nil)

	if err != nil {
		return nil, fmt.Errorf("Could not get tweets")
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", plugin.Token))
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Could not get tweets: ", err.Error())
	}

	defer res.Body.Close()

	var result []Tweet
	err = json.NewDecoder(res.Body).Decode(&result)

	if err != nil {
		return nil, fmt.Errorf("Could not read tweets: ", err.Error())
	}

	return result, nil
}
