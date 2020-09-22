package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	Info  *log.Logger
	Error *log.Logger
)

type RateLimitResponse struct {
	Delay int `json:"retry_after"`
}

func initLog() {
	Info = log.New(os.Stdout,
		"[INFO] ",
		log.Ldate|log.Ltime)

	Error = log.New(os.Stderr,
		"[ERROR] ",
		log.Ldate|log.Ltime)
}

func main() {
	initLog()

	Info.Println("Checking for updates...")

	client := &DiscordClient{os.Getenv("DISCORD_WEBHOOK")}
	var wg sync.WaitGroup

	wg.Add(2)

	erroredChannel := make(chan interface{})

	go CheckProgress(client, &wg, erroredChannel)
	go CheckTwitter(client, &wg, erroredChannel)

	errored := false

	go func() {
		for {
			select {
			case <-erroredChannel:
				errored = true
			}
		}
	}()

	wg.Wait()

	if errored {
		Error.Fatal("Errors occurred while trying to check for updates")
	}
}

type DiscordClient struct {
	webhookUrl string
}

func (discord *DiscordClient) Send(text, name, avatar string, embed interface{}) {
	discord.trySend(text, name, avatar, embed, 0)
}

func (discord *DiscordClient) trySend(text, name, avatar string, embed interface{}, try int) {
	body := map[string]interface{}{
		"username":   name,
		"avatar_url": avatar,
		"content":    text,
	}

	if embed != nil {
		body["embeds"] = []interface{}{embed}
	}

	serialized, err := json.Marshal(body)
	if err != nil {
		Error.Fatal(err)
	}

	res, err := http.Post(discord.webhookUrl, "application/json", bytes.NewReader(serialized))
	if err != nil {
		Error.Fatal(err)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		Error.Fatal(err)
	}

	if res.StatusCode == http.StatusTooManyRequests {
		if try == 2 {
			Error.Println("Couldn't send Discord message: Rate limiting still applied after 3 retries")
		}

		var data RateLimitResponse
		if err := json.Unmarshal(responseBody, &data); err != nil {
			Error.Fatal(err)
		}

		Info.Printf("Being rate late limited by Discord, waiting for %dms\n", data.Delay)
		time.Sleep(time.Duration(data.Delay) * time.Millisecond)

		discord.trySend(text, name, avatar, embed, try+1)
		return
	}

	if res.StatusCode != http.StatusNoContent {
		Error.Fatal("Couldn't send Discord message: ", string(responseBody))
	}
}
