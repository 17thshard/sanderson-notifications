package plugins

import (
	"context"
	"log"
	"net/http"
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type DiscordSender interface {
	Send(text, name, avatar string, embed interface{}) error
	SendWithCustomAvatar(text, name, avatarURL string, embed interface{}) error
}

type Plugin interface {
	Name() string

	Validate() error

	OffsetPrototype() interface{}

	Check(offset interface{}, ctx PluginContext) (interface{}, error)
}

type PluginContext struct {
	Discord    DiscordSender
	Info       *log.Logger
	Error      *log.Logger
	Context    *context.Context
	HTTPClient HTTPClient
}
