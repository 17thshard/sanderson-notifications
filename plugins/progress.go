package plugins

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type ProgressPlugin struct {
	Url           string
	Message       string
	DebounceDelay time.Duration

	clock    Clock
	renderer ProgressDiffRenderer
}

func (plugin *ProgressPlugin) Name() string {
	return "progress"
}

func (plugin *ProgressPlugin) Validate() error {
	if len(plugin.Url) == 0 {
		return fmt.Errorf("URL for progress updates must not be empty")
	}

	if len(plugin.Message) == 0 {
		return fmt.Errorf("message for progress updates must not be empty")
	}

	if plugin.DebounceDelay < 0 {
		return fmt.Errorf("debounce delay must not be negative")
	}

	return nil
}

type ProgressOffset struct {
	Published     []Progress // What Discord users have seen
	Observed      []Progress // What we've observed from website
	DebounceStart *time.Time // When debounce period started (nil = not debouncing)
}

// UnmarshalJSON handles both new format and legacy format
func (p *ProgressOffset) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as new format first
	type V2 ProgressOffset
	v2 := &struct {
		*V2
	}{
		V2: (*V2)(p),
	}

	if err := json.Unmarshal(data, v2); err == nil {
		// Successfully parsed as new format
		return nil
	}

	// Try legacy format ([]Progress)
	var legacyProgress []Progress
	if err := json.Unmarshal(data, &legacyProgress); err != nil {
		return err
	}

	// Convert legacy format to new format
	p.Published = legacyProgress
	p.Observed = legacyProgress
	p.DebounceStart = nil
	return nil
}

func (plugin *ProgressPlugin) OffsetPrototype() interface{} {
	return ProgressOffset{}
}

func (plugin *ProgressPlugin) Init() error {
	plugin.clock = systemClock{}
	plugin.renderer = progressRenderer{}

	return nil
}

type Progress struct {
	Title string
	Link  string
	Value int
}

type ProgressDiff struct {
	Title    string
	Link     string
	OldValue int
	Value    int
	New      bool
}

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func (plugin *ProgressPlugin) Check(rawOffset interface{}, context PluginContext) (interface{}, error) {
	context.Info.Println("Checking for progress updates...")

	var offset ProgressOffset
	if rawOffset != nil {
		offset = rawOffset.(ProgressOffset)
	}

	res, err := context.HTTP.Get(plugin.Url)
	if err != nil {
		return rawOffset, fmt.Errorf("could not read progress site '%s': %w", plugin.Url, err)
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return rawOffset, err
	}

	currentProgress, err := readProgress(doc)
	if err != nil {
		return rawOffset, err
	}
	now := plugin.clock.Now()

	// If we do not detect any changes compared to the last published state, we have entered a (new) stable period.
	// So just reset any potential debouncing
	changes := diff(offset.Published, currentProgress)
	if changes == nil {
		context.Info.Println("No progress changes detected, clearing any debouncing...")
		return ProgressOffset{
			Published: offset.Published,
			Observed:  currentProgress,
		}, nil
	}

	if plugin.shouldDelay(now, offset) {
		newOffset := ProgressOffset{
			Published:     offset.Published,
			Observed:      currentProgress,
			DebounceStart: offset.DebounceStart,
		}

		switch {
		case offset.DebounceStart == nil:
			context.Info.Println("Progress changes detected, starting debounce timer...")
			newOffset.DebounceStart = &now
		case diff(offset.Observed, currentProgress) != nil:
			context.Info.Println("Progress on website has changed again since last run, resetting debounce timer...")
			newOffset.DebounceStart = &now
		default:
			context.Info.Println("Progress changes detected, delaying report to debounce result...")
		}

		return newOffset, nil
	}

	context.Info.Println("Debounce period is over. Reporting changed progress bars...")

	if err = plugin.reportProgress(context.Discord, changes); err != nil {
		return offset, err
	}

	return ProgressOffset{
		Published: currentProgress,
		Observed:  currentProgress,
	}, nil
}

func (plugin *ProgressPlugin) shouldDelay(now time.Time, offset ProgressOffset) bool {
	switch {
	case plugin.DebounceDelay == 0:
		return false // Immediate mode
	case offset.DebounceStart == nil:
		return true
	default:
		return now.Sub(*offset.DebounceStart) < plugin.DebounceDelay
	}
}

func readProgress(doc *goquery.Document) ([]Progress, error) {
	bars := doc.Find("[class^=progress-item-template]")
	result := make([]Progress, bars.Length())

	if bars.Length() == 0 {
		html, _ := doc.Html()
		return nil, fmt.Errorf("Unexpectedly received empty list of progress bars, content was %s", html)
	}

	bars.Each(func(i int, selection *goquery.Selection) {
		title := strings.TrimSpace(selection.Find("[class^=progress-title-template]").Text())
		link := selection.Find("a").AttrOr("href", "")
		value := strings.TrimSuffix(selection.Find("[class^=progress-percent-template]").Text(), "%")

		parsedValue, _ := strconv.Atoi(value)

		result[i] = Progress{title, link, parsedValue}
	})

	return result, nil
}

func diff(old, new []Progress) []ProgressDiff {
	result := make([]ProgressDiff, len(new), len(new))
	oldKeyed := make(map[string]Progress)

	for _, v := range old {
		oldKeyed[v.Title] = v
	}

	noChanges := true
	for i, v := range new {
		existing, existedBefore := oldKeyed[v.Title]

		oldValue := 0
		if existedBefore {
			oldValue = existing.Value
		}

		result[i] = ProgressDiff{
			v.Title,
			v.Link,
			oldValue,
			v.Value,
			!existedBefore,
		}

		if !existedBefore || oldValue != v.Value {
			noChanges = false
		}
	}

	if noChanges {
		return nil
	}

	return result
}

func (plugin *ProgressPlugin) reportProgress(client DiscordSender, progressBars []ProgressDiff) error {
	embed := map[string]interface{}{
		"description": plugin.renderer.Render(progressBars),
		"footer": map[string]interface{}{
			"text": fmt.Sprintf("See %s for more", plugin.Url),
		},
	}

	return client.Send(
		plugin.Message,
		"Progress Updates",
		"dragonsteel",
		embed,
	)
}

type ProgressDiffRenderer interface {
	Render(progressBars []ProgressDiff) string
}

const (
	blockSize  = 2.5
	blockCount = 100 / blockSize
)

type progressRenderer struct {
}

func (r progressRenderer) Render(progressBars []ProgressDiff) string {
	var builder strings.Builder

	for i, progress := range progressBars {
		if i != 0 {
			builder.WriteString("\n\n")
		}

		title := progress.Title
		if len(progress.Link) > 0 {
			title = fmt.Sprintf("[%s](%s)", progress.Title, progress.Link)
		}
		if progress.New {
			title = fmt.Sprintf("[New] %s", title)
		} else if progress.Value != progress.OldValue {
			title = fmt.Sprintf("[Changed] %s (%d%% → %d%%)", title, progress.OldValue, progress.Value)
		}
		builder.WriteString(fmt.Sprintf("**%s**\n", title))

		fullBlocks := int(math.Floor(float64(progress.Value) / blockSize))
		builder.WriteRune('`')
		builder.WriteString(strings.Repeat("█", fullBlocks))
		builder.WriteString(strings.Repeat("░", blockCount-fullBlocks))
		builder.WriteString(fmt.Sprintf(" %3d%%", progress.Value))
		builder.WriteRune('`')
	}

	return builder.String()
}
