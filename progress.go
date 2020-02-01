package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Progress struct {
	Title string
	Link  string
	Value int
}

const (
	blockSize  = 2.5
	blockCount = 100 / blockSize
)

func CheckProgress(client *DiscordClient) {
	res, err := http.Get("https://brandonsanderson.com")
	if err != nil {
		log.Fatal("Could not read Brandon's blog", err.Error())
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	oldProgress := readOldProgress()
	currentProgress := readProgress(doc)

	if equal(oldProgress, currentProgress) {
		log.Println("No progress changes to report.")
		return
	}

	log.Println("Reporting changed progress bars...")

	reportProgress(client, currentProgress)

	err = persistProgress(currentProgress)
	if err != nil {
		log.Fatal(err)
	}
}

func readOldProgress() []Progress {
	content, err := ioutil.ReadFile("last_progress.json")
	if os.IsNotExist(err) {
		content = []byte("[]")
	}

	var oldProgress []Progress
	err = json.Unmarshal(content, &oldProgress)
	if err != nil {
		log.Fatal(err)
	}

	return oldProgress
}

func readProgress(doc *goquery.Document) []Progress {
	bars := doc.Find(".vc_progress_bar .vc_label")
	result := make([]Progress, bars.Length())

	bars.Each(func(i int, selection *goquery.Selection) {
		title := strings.TrimSpace(selection.Contents().Not("span").Text())
		link := selection.Find("a").AttrOr("href", "")
		value := selection.NextFiltered(".vc_single_bar").Find(".vc_bar").AttrOr("data-percentage-value", "0")

		parsedValue, _ := strconv.Atoi(value)

		result[i] = Progress{title, link, parsedValue}
	})

	return result
}

func equal(a, b []Progress) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

func reportProgress(client *DiscordClient, progressBars []Progress) {
	var embedBuilder strings.Builder

	for i, progress := range progressBars {
		if i != 0 {
			embedBuilder.WriteString("\n\n")
		}

		title := progress.Title
		if len(progress.Link) > 0 {
			title = fmt.Sprintf("[%s](%s)", progress.Title, progress.Link)
		}
		embedBuilder.WriteString(fmt.Sprintf("**%s**\n", title))

		fullBlocks := int(math.Floor(float64(progress.Value) / blockSize))
		embedBuilder.WriteRune('`')
		embedBuilder.WriteString(strings.Repeat("█", fullBlocks))
		embedBuilder.WriteString(strings.Repeat("░", blockCount - fullBlocks))
		embedBuilder.WriteString(fmt.Sprintf(" %3d%%", progress.Value))
		embedBuilder.WriteRune('`')
	}

	embed := map[string]interface{}{
		"description": embedBuilder.String(),
		"footer": map[string]interface{}{
			"text": "See https://brandonsanderson.com for more",
		},
	}

	client.Send(
		"There just has been an update to the progress bars on Brandon's website!",
		"Progress Updates",
		"https://www.17thshard.com/forum/uploads/monthly_2017_12/Dragonsteelblack.png.500984e8ce0aad0dce1c7fb779b90c44.png",
		embed,
	)
}

func persistProgress(progress []Progress) error {
	content, _ := json.Marshal(progress)

	return ioutil.WriteFile("last_progress.json", content, 0644)
}
