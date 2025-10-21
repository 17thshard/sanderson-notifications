package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"

	. "17thshard.com/sanderson-notifications/common"
	. "17thshard.com/sanderson-notifications/plugins"
)

func main() {
	infoLog, errorLog := CreateLoggers("main")

	configPath := flag.String("config", "config.yml", "path of YAML config file")
	offsetsPath := flag.String("offsets", "offsets.json", "path of offset storage file")
	flag.Parse()

	configLoader := ConfigLoader{
		AvailablePlugins: map[string]func() Plugin{
			"atom": func() Plugin {
				return &AtomPlugin{}
			},
			"progress": func() Plugin {
				return &ProgressPlugin{}
			},
			"twitter": func() Plugin {
				return &TwitterPlugin{}
			},
			"youtube": func() Plugin {
				return &YouTubePlugin{}
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

	offsetContent, err := os.ReadFile(*offsetsPath)
	if os.IsNotExist(err) {
		offsetContent = []byte("{}")
	} else if err != nil {
		errorLog.Fatalf("Failed to load offsets: %s", err)
	}

	var rawOffsets map[string]json.RawMessage
	if err = json.Unmarshal(offsetContent, &rawOffsets); err != nil {
		errorLog.Fatalf("Failed to parse offsets: %s", err)
	}

	infoLog.Println("Checking for updates...")

	client := CreateDiscordClient(config.DiscordWebhook, config.DiscordMentions)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(len(config.Connectors))
	erroredChannel := make(chan interface{})
	var workingOffsets sync.Map

	// Store old offsets, so they're not lost in case of failure or between config changes
	for connector, offset := range rawOffsets {
		workingOffsets.Store(connector, offset)
	}

	for _, connector := range config.Connectors {
		connector := connector

		connectorInfo, connectorError := CreateLoggers(fmt.Sprintf("connector=%s", connector.Name))
		pluginContext := PluginContext{
			Discord: &client,
			Info:    connectorInfo,
			Error:   connectorError,
			Context: &ctx,
			HTTP:    &http.Client{},
		}

		err = (*connector.Plugin).Init()
		if err != nil {
			pluginContext.Error.Printf("Init for connector '%s' failed: %s", connector.Name, err)
			continue
		}

		go func() {
			defer wg.Done()
			var offset interface{}
			rawOffset, ok := rawOffsets[connector.Name]
			if ok {
				offsetPrototype := (*connector.Plugin).OffsetPrototype()
				offsetRef := reflect.New(reflect.TypeOf(offsetPrototype))
				offsetRef.Elem().Set(reflect.ValueOf(offsetPrototype))
				if err = json.Unmarshal(rawOffset, offsetRef.Interface()); err != nil {
					pluginContext.Error.Printf("Could not parse offsets for connector '%s': %s", connector.Name, err)
					erroredChannel <- nil
					return
				}
				offset = offsetRef.Elem().Interface()
			}

			newOffset, err := (*connector.Plugin).Check(offset, pluginContext)
			if err != nil {
				pluginContext.Error.Printf("Check for connector '%s' failed: %s", connector.Name, err)
				erroredChannel <- nil
			}
			workingOffsets.Store(connector.Name, newOffset)
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

	infoLog.Println("Storing new offsets...")
	newOffsets := make(map[string]interface{})
	workingOffsets.Range(func(k interface{}, v interface{}) bool {
		newOffsets[k.(string)] = v
		return true
	})
	serializedOffsets, err := json.Marshal(newOffsets)
	if err != nil {
		errorLog.Fatalf("Failed to serialize new offsets: %s", err)
	}

	if err = os.WriteFile(*offsetsPath, serializedOffsets, 0644); err != nil {
		errorLog.Fatalf("Failed to write new offsets: %s", err)
	}

	if errored {
		errorLog.Fatal("Errors occurred while trying to check for updates")
	}
}
