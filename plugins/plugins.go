package plugins

import (
	"context"
	"log"
	"net/http"
)

type HTTPClient interface {
	Head(url string) (*http.Response, error)
	Get(url string) (*http.Response, error)
}

type DiscordSender interface {
	Send(text, name, avatar string, embed interface{}) error
	SendWithCustomAvatar(text, name, avatarURL string, embed interface{}) error
}

type Plugin interface {
	Name() string

	// Validate ensures plugin configuration is correct
	Validate() error

	// OffsetPrototype should return an object that the JSON offset can be unmarshaled into
	OffsetPrototype() interface{}

	// Init performs any additional work that needs to be done before the plugin's checks can run
	Init() error

	// Check receives the current offset for the plugin and must return the new offset to be used for the next run
	Check(offset interface{}, ctx PluginContext) (interface{}, error)
}

type PluginContext struct {
	Discord DiscordSender
	Info    *log.Logger
	Error   *log.Logger
	Context *context.Context
	HTTP    HTTPClient
}
