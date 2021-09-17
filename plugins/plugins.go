package plugins

import (
	"17thshard.com/sanderson-notifications/common"
	"log"
)

type Plugin interface {
	Name() string

	Check(ctx PluginContext) error
}

type PluginContext struct {
	Discord *common.DiscordClient
	Info    *log.Logger
	Error   *log.Logger
}
