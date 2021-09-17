package common

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type DiscordClient struct {
	webhookUrl string
	info       *log.Logger
	error      *log.Logger
}

func CreateDiscordClient(webhookUrl string) DiscordClient {
	return DiscordClient{
		webhookUrl: webhookUrl,
		info: log.New(os.Stdout,
			"[INFO] [discord-client]",
			log.Ldate|log.Ltime),
		error: log.New(os.Stderr,
			"[ERROR] [discord-client]",
			log.Ldate|log.Ltime),
	}
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
		discord.error.Fatal(err)
	}

	res, err := http.Post(discord.webhookUrl, "application/json", bytes.NewReader(serialized))
	if err != nil {
		discord.error.Fatal(err)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		discord.error.Fatal(err)
	}

	if res.StatusCode == http.StatusTooManyRequests {
		if try == 2 {
			discord.error.Println("Couldn't send Discord message: Rate limiting still applied after 3 retries")
		}

		var data RateLimitResponse
		if err := json.Unmarshal(responseBody, &data); err != nil {
			discord.error.Fatal(err)
		}

		discord.info.Printf("Being rate late limited by Discord, waiting for %dms\n", data.Delay)
		time.Sleep(time.Duration(data.Delay) * time.Millisecond)

		discord.trySend(text, name, avatar, embed, try+1)
		return
	}

	if res.StatusCode != http.StatusNoContent {
		discord.error.Fatal("Couldn't send Discord message: ", string(responseBody))
	}
}

type RateLimitResponse struct {
	Delay int `json:"retry_after"`
}
