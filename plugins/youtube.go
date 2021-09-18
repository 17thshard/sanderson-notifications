package plugins

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"net/http"
	"net/url"
)

type YouTubePlugin struct {
	ChannelId string `mapstructure:"channelId"`
	Nickname  string
	Message   string
}

func (plugin YouTubePlugin) Name() string {
	return "youtube"
}

func (plugin YouTubePlugin) Validate() error {
	if len(plugin.ChannelId) == 0 {
		return fmt.Errorf("channel ID for YouTube must not be empty")
	}

	if len(plugin.Nickname) == 0 && len(plugin.Message) == 0 {
		return fmt.Errorf("either a channel nickname or a YouTube post message must be given")
	}

	return nil
}

func (plugin YouTubePlugin) OffsetPrototype() interface{} {
	return map[string]bool{}
}

type YouTubePost struct {
	ID    string
	Title string
	Link  string
}

func (plugin YouTubePlugin) Check(offset interface{}, context PluginContext) (interface{}, error) {
	context.Info.Println("Checking for YouTube updates...")

	res, err := http.Get(fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", url.QueryEscape(plugin.ChannelId)))
	if err != nil {
		return offset, fmt.Errorf("could not read YouTube feed for channel '%s': %w", plugin.ChannelId, err)
	}
	defer res.Body.Close()

	fp := atom.Parser{}
	atomFeed, err := fp.Parse(res.Body)
	if err != nil {
		return offset, err
	}

	if len(atomFeed.Entries) == 0 {
		context.Info.Println("No entries in YouTube feed.")
		return offset, nil
	}

	handledEntries := make(map[string]bool)
	if offset != nil {
		handledEntries = offset.(map[string]bool)
	}

	var sortedEntries []YouTubePost

	for _, entry := range atomFeed.Entries {
		if handled, present := handledEntries[entry.ID]; present && handled {
			continue
		}

		sortedEntries = append([]YouTubePost{{
			ID:    entry.ID,
			Title: entry.Title,
			Link:  entry.Links[0].Href,
		}}, sortedEntries...)
	}

	if len(sortedEntries) == 0 {
		context.Info.Println("No YouTube posts to report.")
		return offset, nil
	}

	context.Info.Println("Reporting YouTube posts...")

	if len(plugin.Nickname) == 0 && len(plugin.Message) == 0 {
		plugin.Nickname = atomFeed.Title
		context.Info.Printf(
			"No nickname or specific messages were provided for channel '%s', using feed title '%s' as fallback nickname",
			plugin.ChannelId,
			plugin.Nickname,
		)
	}

	for _, entry := range sortedEntries {
		message := fmt.Sprintf("%s posted something on YouTube", plugin.Nickname)
		if len(plugin.Message) > 0 {
			message = plugin.Message
		}

		if err = context.Discord.Send(
			fmt.Sprintf("%s %s", message, entry.Link),
			"YouTube",
			"youtube",
			nil,
		); err != nil {
			return handledEntries, err
		}

		handledEntries[entry.ID] = true

		context.Info.Println("Reported YouTube post ", entry.Title)
	}

	return handledEntries, nil
}
