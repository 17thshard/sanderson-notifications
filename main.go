package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	client := &DiscordClient{os.Getenv("DISCORD_WEBHOOK")}
	CheckProgress(client)
	CheckTwitter(client)
}

type DiscordClient struct {
	webhookUrl string
}

func (discord *DiscordClient) Send(text, name, avatar string, embed interface{}) {
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
		log.Fatal(err)
	}

	res, err := http.Post(discord.webhookUrl, "application/json", bytes.NewReader(serialized))
	if err != nil {
		log.Fatal(err)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	if res.StatusCode != http.StatusOK {
		log.Fatal("Couldn't send Discord message:", string(responseBody))
	}
}
