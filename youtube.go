package main

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

type YouTubePost struct {
	Title string
	Link  string
}

func CheckYouTube(client *DiscordClient, wg *sync.WaitGroup, errored chan interface{}) {
	defer wg.Done()

	Info.Println("Checking for YouTube updates...")

	res, err := http.Get("https://www.youtube.com/feeds/videos.xml?channel_id=UC3g-w83Cb5pEAu5UmRrge-A")
	if err != nil {
		Error.Println("Could not read Brandon's YouTube feed", err.Error())
		errored <- nil
		return
	}
	defer res.Body.Close()

	fp := atom.Parser{}
	atomFeed, err := fp.Parse(res.Body)
	if err != nil {
		Error.Println(err)
		errored <- nil
		return
	}

	if len(atomFeed.Entries) == 0 {
		Info.Println("No entries in YouTube feed.")
		return
	}

	lastEntryId := readLastFeedEntryId()

	var entries []YouTubePost

	for _, entry := range atomFeed.Entries {
		if entry.ID == lastEntryId {
			break
		}

		entries = append([]YouTubePost{{
			Title: entry.Title,
			Link:  entry.Links[0].Href,
		}}, entries...)
	}

	if len(entries) == 0 {
		Info.Println("No YouTube posts to report.")
		return
	}

	Info.Println("Reporting YouTube posts...")

	for _, entry := range entries {
		message := "Brandon just posted something on YouTube"

		client.Send(
			fmt.Sprintf("%s %s", message, entry.Link),
			"YouTube",
			"https://upload.wikimedia.org/wikipedia/commons/thumb/4/4c/YouTube_icon.png/640px-YouTube_icon.png",
			nil,
		)

		Info.Println("Reported YouTube post ", entry.Title)
	}

	// First entry is "last" as in newest
	err = persistLastFeedEntryId(atomFeed.Entries[0].ID)
	if err != nil {
		Error.Println(err)
		errored <- nil
		return
	}
}

func readLastFeedEntryId() string {
	content, err := ioutil.ReadFile("last_yt_feed_entry")
	if os.IsNotExist(err) {
		return ""
	}

	return strings.TrimSpace(string(content))
}

func persistLastFeedEntryId(id string) error {
	return ioutil.WriteFile("last_yt_feed_entry", []byte(id), 0644)
}
