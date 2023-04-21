package plugins

import (
	goContext "context"
	"fmt"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

type TwitterPlugin struct {
	Account                 string
	Nickname                string
	TweetMessage            string   `mapstructure:"tweetMessage"`
	RetweetMessage          string   `mapstructure:"retweetMessage"`
	ExcludedRetweetAccounts []string `mapstructure:"excludeRetweetsOf"`
	retweetExclusions       map[string]bool
	scraper                 *twitterscraper.Scraper
}

func (plugin *TwitterPlugin) Name() string {
	return "twitter"
}

func (plugin *TwitterPlugin) Validate() error {
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

	plugin.scraper = twitterscraper.New().WithReplies(true)

	tweets, err := plugin.retrieveTweetsSince(lastTweet)
	if err != nil {
		return lastTweet, err
	}

	if len(tweets) == 0 {
		context.Info.Println("No tweets to report.")
		return lastTweet, nil
	}

	context.Info.Printf("Reporting %d tweets...\n", len(tweets))

	if len(plugin.Nickname) == 0 && (len(plugin.TweetMessage) == 0 || len(plugin.RetweetMessage) == 0) {
		profile, err := plugin.scraper.GetProfile(plugin.Account)

		if err != nil {
			return lastTweet, err
		}

		plugin.Nickname = profile.Name
		context.Info.Printf(
			"No nickname or specific messages were provided for account '%s', using name '%s' as fallback nickname",
			plugin.Account,
			plugin.Nickname,
		)
	}

	for i := len(tweets) - 1; i >= 0; i-- {
		tweet := tweets[i]
		if tweet.RetweetedStatus != nil {
			if exclude, present := plugin.retweetExclusions[tweet.RetweetedStatus.Username]; present && exclude {
				context.Info.Printf(
					"Ignoring retweet %s from '%s', as the original tweet is from '%s'",
					tweet.ID,
					tweet.Username,
					tweet.RetweetedStatus.Username,
				)
				lastTweet = tweet.ID
				continue
			}
		}

		if tweet.IsReply && (tweet.InReplyToStatus == nil || tweet.InReplyToStatus.Username != plugin.Account) {
			context.Info.Printf(
				"Ignoring reply tweet %s from '%s', as it is not in response to themself",
				tweet.ID,
				tweet.Username,
			)
			lastTweet = tweet.ID
			continue
		}

		messageTweet := tweet
		message := fmt.Sprintf("%s tweeted", plugin.Nickname)
		if len(plugin.TweetMessage) > 0 {
			message = plugin.TweetMessage
		}
		if tweet.RetweetedStatus != nil {
			messageTweet = *tweet.RetweetedStatus
			message = fmt.Sprintf("%s retweeted", plugin.Nickname)
			if len(plugin.RetweetMessage) > 0 {
				message = plugin.RetweetMessage
			}
		}

		text := fmt.Sprintf("%s https://twitter.com/%s/status/%s", message, messageTweet.Username, messageTweet.ID)
		if tweet.RetweetedStatus != nil {
			text = fmt.Sprintf(
				"%s (https://twitter.com/%s/status/%s)",
				text,
				tweet.Username,
				tweet.ID,
			)
		}

		if err = context.Discord.Send(
			text,
			"Twitter",
			"twitter",
			nil,
		); err != nil {
			return lastTweet, err
		}

		lastTweet = tweet.ID
	}

	return lastTweet, nil
}

func (plugin *TwitterPlugin) retrieveTweetsSince(lastTweet string) ([]twitterscraper.Tweet, error) {
	var result []twitterscraper.Tweet

	for tweet := range plugin.scraper.GetTweets(goContext.Background(), plugin.Account, 3200) {
		if tweet.ID == lastTweet {
			break
		}

		if tweet.Error != nil {
			return nil, fmt.Errorf("could not read tweets: %w", tweet.Error)
		}

		result = append(result, tweet.Tweet)
	}

	return result, nil
}
