package main

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
)

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

	for _, entry := range atomFeed.Entries {
		if entry.ID == lastEntryId {
			break
		}

		message := "Brandon just posted something on YouTube"

		client.Send(
			fmt.Sprintf("%s %s", message, entry.Links[0].Href),
			"YouTube",
			"https://upload.wikimedia.org/wikipedia/commons/thumb/4/4c/YouTube_icon.png/640px-YouTube_icon.png",
			nil,
		)
	}

	Info.Println("Reporting changed progress bars...")

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

	return string(content)
}

func persistLastFeedEntryId(id string) error {
	return ioutil.WriteFile("last_yt_feed_entry", []byte(id), 0644)
}
