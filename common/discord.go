package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const webhookBaseUrl = "https://discord.com/api/webhooks"
const avatarBaseUrl = "https://raw.githubusercontent.com/Palanaeum/sanderson-notifications/master/avatars"
const maxRetries = 3

type DiscordClient struct {
	webhookUrl string
	info       *log.Logger
	error      *log.Logger
}

func CreateDiscordClient(webhook string) DiscordClient {
	infoLog, errorLog := CreateLoggers("main")

	return DiscordClient{
		webhookUrl: fmt.Sprintf("%s/%s", webhookBaseUrl, webhook),
		info:       infoLog,
		error:      errorLog,
	}
}

func (discord *DiscordClient) Send(text, name, avatar string, embed interface{}) error {
	return discord.trySend(text, name, avatar, embed, 1)
}

func (discord *DiscordClient) trySend(text, name, avatar string, embed interface{}, try int) error {
	body := map[string]interface{}{
		"username":   name,
		"avatar_url": fmt.Sprintf("%s/%s.png", avatarBaseUrl, avatar),
		"content":    text,
	}

	if embed != nil {
		body["embeds"] = []interface{}{embed}
	}

	serialized, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("could not serialize request: %w", err)
	}

	res, err := http.Post(discord.webhookUrl, "application/json", bytes.NewReader(serialized))
	if err != nil {
		return fmt.Errorf("could not send Discord request: %w", err)
	}

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("could not read Discord response: %w", err)
	}

	if res.StatusCode == http.StatusTooManyRequests {
		if try == maxRetries {
			return fmt.Errorf("couldn't send Discord message: Rate limiting still applied after %d retries", maxRetries)
		}

		var data RateLimitResponse
		if err := json.Unmarshal(responseBody, &data); err != nil {
			return fmt.Errorf("could not parse Discord response: %w", err)
		}

		discord.info.Printf("Being rate late limited by Discord, waiting for %dms\n", data.Delay)
		time.Sleep(time.Duration(data.Delay) * time.Millisecond)

		return discord.trySend(text, name, avatar, embed, try+1)
	}

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("couldn't send Discord message: %s", string(responseBody))
	}

	return nil
}

type RateLimitResponse struct {
	Delay int `json:"retry_after"`
}
