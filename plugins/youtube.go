package plugins

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
)

type YouTubePlugin struct {
	ChannelId string `yaml:"channelId"`
	Nickname  string
	Message   string
}

const feedIdDir = "youtube_feed_entries"

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

type YouTubePost struct {
	ID    string
	Title string
	Link  string
}

func (plugin YouTubePlugin) Check(context PluginContext) error {
	context.Info.Println("Checking for YouTube updates...")

	res, err := http.Get(fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", url.QueryEscape(plugin.ChannelId)))
	if err != nil {
		return fmt.Errorf("could not read YouTube feed for channel '%s': %w", plugin.ChannelId, err)
	}
	defer res.Body.Close()

	fp := atom.Parser{}
	atomFeed, err := fp.Parse(res.Body)
	if err != nil {
		return err
	}

	if len(atomFeed.Entries) == 0 {
		context.Info.Println("No entries in YouTube feed.")
		return nil
	}

	var sortedEntries []YouTubePost

	for _, entry := range atomFeed.Entries {
		if hasHandledFeedEntryId(entry.ID) {
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
		return nil
	}

	context.Info.Println("Reporting YouTube posts...")

	for _, entry := range sortedEntries {
		message := fmt.Sprintf("%s posted something on YouTube", plugin.Nickname)
		if len(plugin.Message) > 0 {
			message = plugin.Message
		}

		context.Discord.Send(
			fmt.Sprintf("%s %s", message, entry.Link),
			"YouTube",
			"https://upload.wikimedia.org/wikipedia/commons/thumb/4/4c/YouTube_icon.png/640px-YouTube_icon.png",
			nil,
		)

		err = persistFeedEntryId(entry.ID)
		if err != nil {
			return err
		}

		context.Info.Println("Reported YouTube post ", entry.Title)
	}

	return nil
}

func hasHandledFeedEntryId(id string) bool {
	if _, err := os.Stat(buildFeedEntryIdPath(id)); os.IsNotExist(err) {
		return false
	}

	return true
}

func persistFeedEntryId(id string) error {
	if _, err := os.Stat(feedIdDir); os.IsNotExist(err) {
		err = os.Mkdir(feedIdDir, os.ModePerm)

		if err != nil {
			return err
		}
	}

	return ioutil.WriteFile(buildFeedEntryIdPath(id), []byte(id), 0644)
}

func buildFeedEntryIdPath(id string) string {
	var re = regexp.MustCompile("[^a-zA-Z0-9.\\-]")

	return fmt.Sprintf("%s/%s", feedIdDir, re.ReplaceAllString(id, "_"))
}
