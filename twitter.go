package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Tweet struct {
	Id              uint64
	RetweetedStatus *Tweet `json:"retweeted_status"`
}

func CheckTwitter(client *DiscordClient, wg *sync.WaitGroup, errored chan interface{}) {
	defer wg.Done()

	Info.Println("Checking for new tweets...")
	twitterToken := os.Getenv("TWITTER_TOKEN")

	if len(twitterToken) == 0 {
		Error.Println("Missing Twitter token")
		errored <- nil
		return
	}

	lastTweet, err := retrieveLastTweet()
	if err != nil {
		Error.Println(err)
		errored <- nil
		return
	}

	tweets, err := retrieveTweetsSince(twitterToken, lastTweet)
	if err != nil {
		Error.Println(err)
		errored <- nil
		return
	}

	if len(tweets) == 0 {
		Info.Println("No tweets to report.")
		return
	}

	Info.Printf("Reporting %d tweets...\n", len(tweets))

	for i := len(tweets) - 1; i >= 0; i-- {
		message := "Brandon just tweeted"
		if tweets[i].RetweetedStatus != nil {
			message = "Brandon just retweeted something"
		}

		client.Send(
			fmt.Sprintf("%s https://twitter.com/BrandSanderson/status/%d", message, tweets[i].Id),
			"Twitter",
			"https://images-na.ssl-images-amazon.com/images/I/31KluT5nBkL._SY355_.png",
			nil,
		)
	}

	err = ioutil.WriteFile("last_tweet", []byte(strconv.FormatUint(tweets[0].Id, 10)), 0644)
	if err != nil {
		Error.Println(err)
		errored <- nil
		return
	}
}

func retrieveLastTweet() (string, error) {
	content, err := ioutil.ReadFile("last_tweet")
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Could not determine last reported tweet")
	}

	return strings.TrimSpace(string(content)), nil
}

func retrieveTweetsSince(token, lastTweet string) ([]Tweet, error) {
	client := &http.Client{}
	timelineUrl := fmt.Sprintf(
		"https://api.twitter.com/1.1/statuses/user_timeline.json"+
			"?screen_name=BrandSanderson"+
			"&since_id=%s"+
			"&exclude_replies=true"+
			"&include_rts=true",
		url.QueryEscape(lastTweet),
	)

	req, err := http.NewRequest("GET", timelineUrl, nil)

	if err != nil {
		return nil, fmt.Errorf("Could not get tweets")
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
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
