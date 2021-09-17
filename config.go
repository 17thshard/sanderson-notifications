package main

import (
	. "17thshard.com/sanderson-notifications/plugins"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type ConfigLoader struct {
	AvailablePlugins map[string]func() Plugin
}

type Config struct {
	Connectors []Connector
}

type Connector struct {
	Name   string
	Plugin Plugin
}

type RawConfig struct {
	Connectors map[string]RawConnector
}

type RawConnector struct {
	Plugin string
}

func (loader ConfigLoader) Load(path string) (*Config, error) {
	configContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	raw := RawConfig{}
	if err = yaml.Unmarshal(configContent, &raw); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	var config Config
	for name, rawConnector := range raw.Connectors {
		pluginBuilder, ok := loader.AvailablePlugins[rawConnector.Plugin]
		if !ok {
			return nil, fmt.Errorf("failed to load connector '%s': unknown plugin '%s'", name, rawConnector.Plugin)
		}

		plugin := pluginBuilder()
		config.Connectors = append(config.Connectors, Connector{Name: name, Plugin: plugin})
	}

	return &config, nil
}
