package plugins

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"math"
	"strconv"
	"strings"
	"time"
)

type ProgressPlugin struct {
	Url           string
	Message       string
	DebounceDelay time.Duration
}

func (plugin ProgressPlugin) Name() string {
	return "progress"
}

func (plugin ProgressPlugin) Validate() error {
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
	PublishedState []Progress  // What Discord users have seen
	ObservedState  []Progress  // What we've observed from website
	DebounceStart  *time.Time  // When debounce period started (nil = not debouncing)
}

// IsDebouncing returns true if we're currently in a debounce period
func (p ProgressOffset) IsDebouncing() bool {
	return p.DebounceStart != nil
}

// DebounceElapsed returns true if the debounce period has elapsed
func (p ProgressOffset) DebounceElapsed(duration time.Duration) bool {
	if !p.IsDebouncing() {
		return false
	}
	return time.Since(*p.DebounceStart) >= duration
}

// HasChanges returns true if there are changes between published and observed state
func (p ProgressOffset) HasChanges() bool {
	changes := diff(p.PublishedState, p.ObservedState)
	return changes != nil
}

// GetPendingChanges returns the diff between published and observed state
func (p ProgressOffset) GetPendingChanges() []ProgressDiff {
	return diff(p.PublishedState, p.ObservedState)
}

// ShouldPublish determines if we should publish changes based on debounce settings
func (p ProgressOffset) ShouldPublish(debounceDelay time.Duration) bool {
	if !p.HasChanges() {
		return false
	}
	if debounceDelay == 0 {
		return true // Immediate mode
	}
	return p.IsDebouncing() && p.DebounceElapsed(debounceDelay)
}

func (p ProgressOffset) ShouldStartDebounce() bool {
	return !p.IsDebouncing()
}

func (p ProgressOffset) ShouldResetDebounce(newOffset ProgressOffset) bool {
	changes := diff(p.ObservedState, newOffset.ObservedState)
	return changes != nil
}

// UnmarshalJSON handles both new format and legacy format
func (p *ProgressOffset) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as new format first
	type Alias ProgressOffset
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	
	if err := json.Unmarshal(data, aux); err == nil {
		// Successfully parsed as new format
		return nil
	}
	
	// Try legacy format ([]Progress)
	var legacyProgress []Progress
	if err := json.Unmarshal(data, &legacyProgress); err != nil {
		return err
	}
	
	// Convert legacy format to new format
	p.PublishedState = legacyProgress
	p.ObservedState = legacyProgress
	p.DebounceStart = nil
	return nil
}

func (plugin ProgressPlugin) OffsetPrototype() interface{} {
	return ProgressOffset{}
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

const (
	blockSize  = 2.5
	blockCount = 100 / blockSize
)

func (plugin ProgressPlugin) Check(offset interface{}, context PluginContext) (interface{}, error) {
	context.Info.Println("Checking for progress updates...")
	
	var oldOffset ProgressOffset
	if offset != nil {
		oldOffset = offset.(ProgressOffset)
	}

	res, err := context.HTTPClient.Get(plugin.Url)
	if err != nil {
		return oldOffset, fmt.Errorf("could not read progress site '%s': %w", plugin.Url, err)
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return oldOffset, err
	}

	currentProgress, err := readProgress(doc)
	if err != nil {
		return oldOffset, err
	}
	now := time.Now()

	// Update observed state
	newOffset := ProgressOffset{
		PublishedState: oldOffset.PublishedState,
		ObservedState:  currentProgress,
		DebounceStart:  oldOffset.DebounceStart,
	}

	if !newOffset.HasChanges() {
		// No changes detected - clear any debouncing
		context.Info.Println("No progress changes detected, clearing any debouncing...")
		return ProgressOffset{
			PublishedState: oldOffset.PublishedState,
			ObservedState:  currentProgress,
		}, nil
	}

	if newOffset.ShouldPublish(plugin.DebounceDelay) {
		// Publish the changes
		context.Info.Println("Reporting changed progress bars...")
		
		if err = plugin.reportProgress(context.Discord, newOffset.GetPendingChanges()); err != nil {
			return oldOffset, err
		}
		
		return ProgressOffset{
			PublishedState: currentProgress, // Update what we published
			ObservedState:  currentProgress,
		}, nil
	}

	// Start or continue debouncing
	if newOffset.ShouldStartDebounce() {
		context.Info.Println("Progress changes detected, starting debounce timer...")
		newOffset.DebounceStart = &now
	} else if newOffset.ShouldResetDebounce(oldOffset) {
		context.Info.Println("Progress changes detected, resetting debounce timer...")
		newOffset.DebounceStart = &now
	} else {
		context.Info.Println("Progress changes detected, continuing debounce timer...")
	}

	return newOffset, nil
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

func (plugin ProgressPlugin) reportProgress(client DiscordSender, progressBars []ProgressDiff) error {
	var embedBuilder strings.Builder

	for i, progress := range progressBars {
		if i != 0 {
			embedBuilder.WriteString("\n\n")
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
		embedBuilder.WriteString(fmt.Sprintf("**%s**\n", title))

		fullBlocks := int(math.Floor(float64(progress.Value) / blockSize))
		embedBuilder.WriteRune('`')
		embedBuilder.WriteString(strings.Repeat("█", fullBlocks))
		embedBuilder.WriteString(strings.Repeat("░", blockCount-fullBlocks))
		embedBuilder.WriteString(fmt.Sprintf(" %3d%%", progress.Value))
		embedBuilder.WriteRune('`')
	}

	embed := map[string]interface{}{
		"description": embedBuilder.String(),
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
