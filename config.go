package main

import (
	. "17thshard.com/sanderson-notifications/plugins"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
	"os"
)

type ConfigLoader struct {
	AvailablePlugins map[string]func() Plugin
}

type Config struct {
	DiscordWebhook      string                            `yaml:"discordWebhook"`
	Connectors          []Connector                       `yaml:"-"`
	SharedPluginConfigs map[string]map[string]interface{} `yaml:"shared"`
	RawConnectors       map[string]RawConnector           `yaml:"connectors"`
}

type Connector struct {
	Name   string
	Plugin Plugin
}

type RawConnector struct {
	Plugin string
	Config map[string]interface{}
}

func (loader ConfigLoader) Load(path string) (*Config, error) {
	configContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var config Config
	if err = yaml.Unmarshal(configContent, &config); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	if len(config.DiscordWebhook) == 0 {
		return nil, fmt.Errorf("config is missing Discord webhook ID")
	}

	for name, rawConnector := range config.RawConnectors {
		pluginBuilder, ok := loader.AvailablePlugins[rawConnector.Plugin]
		if !ok {
			return nil, fmt.Errorf("failed to load connector '%s': unknown plugin '%s'", name, rawConnector.Plugin)
		}

		plugin := pluginBuilder()
		sharedConfig := config.SharedPluginConfigs[rawConnector.Plugin]
		rawConnector.Config = mergeKeys(rawConnector.Config, sharedConfig)
		if len(rawConnector.Config) > 0 {
			if err = mapstructure.Decode(rawConnector.Config, &plugin); err != nil {
				return nil, fmt.Errorf("could not parse config for connector '%s' with plugin '%s': %w", name, rawConnector.Plugin, err)
			}
		}
		if err = plugin.Validate(); err != nil {
			return nil, fmt.Errorf("invalid configuration for connector '%s' with plugin '%s': %w", name, rawConnector.Plugin, err)
		}

		config.Connectors = append(config.Connectors, Connector{Name: name, Plugin: plugin})
	}

	return &config, nil
}

type m = map[string]interface{}

// Given two maps, recursively merge right into left, NEVER replacing any key that already exists in left
func mergeKeys(left, right m) m {
	if left == nil {
		return right
	}

	for key, rightVal := range right {
		if leftVal, present := left[key]; present {
			//then we don't want to replace it - recurse
			left[key] = mergeKeys(leftVal.(m), rightVal.(m))
		} else {
			// key not in left so we can just shove it in
			left[key] = rightVal
		}
	}
	return left
}
