package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type Tweet struct {
	Id uint64
}

func CheckTwitter(client *DiscordClient) {
	Info.Println("Checking for new tweets...")
	twitterToken := os.Getenv("TWITTER_TOKEN")

	if len(twitterToken) == 0 {
		Error.Fatal("Missing Twitter token")
	}

	lastTweet := retrieveLastTweet()
	tweets := retrieveTweetsSince(twitterToken, lastTweet)

	if len(tweets) == 0 {
		Info.Println("No tweets to report.")
		return
	}

	Info.Printf("Reporting %d tweets...\n", len(tweets))

	for i := len(tweets) - 1; i >= 0; i-- {
		client.Send(
			fmt.Sprintf("[](https://twitter.com/BrandSanderson/status/%d )", tweets[i].Id),
			"Twitter",
			"https://images-na.ssl-images-amazon.com/images/I/31KluT5nBkL._SY355_.png",
			nil,
		)
	}

	err := ioutil.WriteFile("last_tweet", []byte(strconv.FormatUint(tweets[0].Id, 10)), 0644)
	if err != nil {
		Error.Fatal(err)
	}
}

func retrieveLastTweet() string {
	content, err := ioutil.ReadFile("last_tweet")
	if os.IsNotExist(err) {
		Error.Fatal("Could not determine last reported tweet")
	}

	return string(content)
}

func retrieveTweetsSince(token, lastTweet string) []Tweet {
	client := &http.Client{}
	timelineUrl := fmt.Sprintf("https://api.twitter.com/1.1/statuses/user_timeline.json?screen_name=BrandSanderson&since_id=%s", url.QueryEscape(lastTweet))

	req, err := http.NewRequest("GET", timelineUrl, nil)

	if err != nil {
		Error.Fatal("Could not get tweets")
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := client.Do(req)
	if err != nil {
		Error.Fatal("Could not get tweets", err.Error())
	}

	defer res.Body.Close()

	var result []Tweet
	err = json.NewDecoder(res.Body).Decode(&result)

	if err != nil {
		Error.Fatal("Could not read tweets", err.Error())
	}

	return result
}
