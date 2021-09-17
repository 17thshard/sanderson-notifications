package plugins

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
)

type YouTubePlugin struct {
}

const feedIdDir = "youtube_feed_entries"

func (plugin YouTubePlugin) Name() string {
	return "youtube"
}

type YouTubePost struct {
	ID    string
	Title string
	Link  string
}

func (plugin YouTubePlugin) Check(context PluginContext) error {
	context.Info.Println("Checking for YouTube updates...")

	res, err := http.Get("https://www.youtube.com/feeds/videos.xml?channel_id=UC3g-w83Cb5pEAu5UmRrge-A")
	if err != nil {
		return fmt.Errorf("could not read Brandons YouTube feed: %w", err)
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
		message := "Brandon just posted something on YouTube"

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
