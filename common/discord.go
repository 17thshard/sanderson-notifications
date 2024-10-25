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
	webhookUrl    string
	mentions      DiscordMentions
	mentionSuffix string
	info          *log.Logger
	error         *log.Logger
}

type DiscordMentions struct {
	Parse []string `json:"parse" yaml:"-"`
	Roles []string `json:"roles" yaml:"roles"`
	Users []string `json:"users" yaml:"users"`
}

func CreateDiscordClient(webhook string, mentions DiscordMentions) DiscordClient {
	infoLog, errorLog := CreateLoggers("main")

	mentionSuffix := ""
	for _, role := range mentions.Roles {
		mentionSuffix += fmt.Sprintf("<@&%s> ", role)
	}
	for _, user := range mentions.Users {
		mentionSuffix += fmt.Sprintf("<@%s> ", user)
	}

	if mentionSuffix != "" {
		mentionSuffix = fmt.Sprintf("\n-# %s", mentionSuffix)
	}

	mentions.Parse = make([]string, 0)

	return DiscordClient{
		webhookUrl:    fmt.Sprintf("%s/%s", webhookBaseUrl, webhook),
		mentions:      mentions,
		mentionSuffix: mentionSuffix,
		info:          infoLog,
		error:         errorLog,
	}
}

func (discord *DiscordClient) Send(text, name, avatar string, embed interface{}) error {
	return discord.trySend(text, name, fmt.Sprintf("%s/%s.png", avatarBaseUrl, avatar), embed, 1)
}

func (discord *DiscordClient) SendWithCustomAvatar(text, name, avatarURL string, embed interface{}) error {
	return discord.trySend(text, name, avatarURL, embed, 1)
}

func (discord *DiscordClient) trySend(text, name, avatarURL string, embed interface{}, try int) error {
	body := map[string]interface{}{
		"username":         name,
		"avatar_url":       avatarURL,
		"content":          fmt.Sprintf("%s%s", text, discord.mentionSuffix),
		"allowed_mentions": discord.mentions,
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

		discord.info.Printf("Being rate late limited by Discord, waiting for %fs\n", data.Delay)
		time.Sleep(time.Duration(data.Delay * float32(time.Second)))

		return discord.trySend(text, name, avatarURL, embed, try+1)
	}

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("couldn't send Discord message: %s", string(responseBody))
	}

	return nil
}

type RateLimitResponse struct {
	Delay float32 `json:"retry_after"`
}
