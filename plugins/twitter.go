package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type TwitterPlugin struct {
	Token          string
	Account        string
	Nickname       string
	TweetMessage   string `yaml:"tweetMessage"`
	RetweetMessage string `yaml:"retweetMessage"`
}

func (plugin TwitterPlugin) Name() string {
	return "twitter"
}

func (plugin TwitterPlugin) Validate() error {
	if len(plugin.Token) == 0 {
		return fmt.Errorf("token for Twitter must not be empty")
	}

	if len(plugin.Nickname) == 0 && (len(plugin.TweetMessage) == 0 || len(plugin.TweetMessage) == 0) {
		return fmt.Errorf("either an account nickname or tweet and retweet messages must be given")
	}

	return nil
}

type Tweet struct {
	Id              uint64
	RetweetedStatus *Tweet `json:"retweeted_status"`
}

func (plugin TwitterPlugin) Check(context PluginContext) error {
	context.Info.Println("Checking for new tweets...")

	lastTweet, err := retrieveLastTweet()
	if err != nil {
		return err
	}

	tweets, err := plugin.retrieveTweetsSince(lastTweet)
	if err != nil {
		return err
	}

	if len(tweets) == 0 {
		context.Info.Println("No tweets to report.")
		return nil
	}

	context.Info.Printf("Reporting %d tweets...\n", len(tweets))

	for i := len(tweets) - 1; i >= 0; i-- {
		message := fmt.Sprintf("%s tweeted", plugin.Nickname)
		if len(plugin.TweetMessage) > 0 {
			message = plugin.TweetMessage
		}
		if tweets[i].RetweetedStatus != nil {
			message = fmt.Sprintf("%s retweeted", plugin.Nickname)
			if len(plugin.RetweetMessage) > 0 {
				message = plugin.RetweetMessage
			}
		}

		context.Discord.Send(
			fmt.Sprintf("%s https://twitter.com/%s/status/%d", message, plugin.Account, tweets[i].Id),
			"Twitter",
			"https://images-na.ssl-images-amazon.com/images/I/31KluT5nBkL._SY355_.png",
			nil,
		)
	}

	err = ioutil.WriteFile("last_tweet", []byte(strconv.FormatUint(tweets[0].Id, 10)), 0644)
	if err != nil {
		return err
	}

	return nil
}

func retrieveLastTweet() (string, error) {
	content, err := ioutil.ReadFile("last_tweet")
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Could not determine last reported tweet")
	}

	return strings.TrimSpace(string(content)), nil
}

func (plugin TwitterPlugin) retrieveTweetsSince(lastTweet string) ([]Tweet, error) {
	client := &http.Client{}
	timelineUrl := fmt.Sprintf(
		"https://api.twitter.com/1.1/statuses/user_timeline.json"+
			"?screen_name=%s"+
			"&since_id=%s"+
			"&exclude_replies=true"+
			"&include_rts=true",
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
