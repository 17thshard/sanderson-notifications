package main

import (
	. "17thshard.com/sanderson-notifications/common"
	. "17thshard.com/sanderson-notifications/plugins"
	"flag"
	"fmt"
	"os"
	"sync"
)

func main() {
	infoLog, errorLog := CreateLoggers("main")

	configPath := flag.String("config", "config.yml", "path of YAML config file")
	flag.Parse()

	configLoader := ConfigLoader{
		AvailablePlugins: map[string]func() Plugin{
			"progress": func() Plugin {
				return ProgressPlugin{}
			},
			"twitter": func() Plugin {
				return TwitterPlugin{}
			},
			"youtube": func() Plugin {
				return YouTubePlugin{}
			},
		},
	}
	config, err := configLoader.Load(*configPath)
	if err != nil {
		errorLog.Fatalf("Failed to load config: %s", err)
	}

	if len(config.Connectors) == 0 {
		errorLog.Println("Config did not contain any connectors. Consider configuring one of the following plugins:")
		for plugin := range configLoader.AvailablePlugins {
			errorLog.Printf(" - %s", plugin)
		}
		os.Exit(1)
	}

	infoLog.Printf("Loaded configuration with %d connectors", len(config.Connectors))
	infoLog.Println("Checking for updates...")

	client := CreateDiscordClient(os.Getenv("DISCORD_WEBHOOK"))

	var wg sync.WaitGroup
	wg.Add(len(config.Connectors))

	erroredChannel := make(chan interface{})

	for _, connector := range config.Connectors {
		connector := connector
		connectorInfo, connectorError := CreateLoggers(fmt.Sprintf("connector=%s", connector.Name))
		context := PluginContext{Discord: &client, Info: connectorInfo, Error: connectorError}
		go func() {
			defer wg.Done()
			if err := connector.Plugin.Check(context); err != nil {
				context.Error.Printf("Check for connector '%s' failed: %s", connector.Name, err)
				erroredChannel <- nil
			}
		}()
	}

	errored := false

	go func() {
		for {
			select {
			case <-erroredChannel:
				errored = true
			}
		}
	}()

	wg.Wait()

	if errored {
		errorLog.Fatal("Errors occurred while trying to check for updates")
	}
}
