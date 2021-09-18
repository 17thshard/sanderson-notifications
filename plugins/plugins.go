package plugins

import (
	"17thshard.com/sanderson-notifications/common"
	"log"
	"reflect"
)

type Plugin interface {
	Name() string

	Validate() error

	OffsetType() reflect.Type

	Check(offset interface{}, ctx PluginContext) (interface{}, error)
}

type PluginContext struct {
	Discord *common.DiscordClient
	Info    *log.Logger
	Error   *log.Logger
}
