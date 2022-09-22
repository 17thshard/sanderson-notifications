package plugins

import (
	"17thshard.com/sanderson-notifications/common"
	"context"
	"log"
)

type Plugin interface {
	Name() string

	Validate() error

	OffsetPrototype() interface{}

	Check(offset interface{}, ctx PluginContext) (interface{}, error)
}

type PluginContext struct {
	Discord *common.DiscordClient
	Info    *log.Logger
	Error   *log.Logger
	Context *context.Context
}
