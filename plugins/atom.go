package plugins

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed/atom"
)

type AtomPlugin struct {
	FeedURL      string `mapstructure:"feedUrl"`
	Nickname     string
	AvatarURL    string `mapstructure:"avatarUrl"`
	Message      string
	ExcludedTags []string       `mapstructure:"excludedTags"`
	MinAge       *time.Duration `mapstructure:"minAge"`
	MaxAge       *time.Duration `mapstructure:"maxAge"`
}

func (plugin *AtomPlugin) Name() string {
	return "atom"
}

func (plugin *AtomPlugin) Validate() error {
	if len(plugin.FeedURL) == 0 {
		return fmt.Errorf("feed URL for Atom integration must not be empty")
	}

	return nil
}

func (plugin *AtomPlugin) OffsetPrototype() interface{} {
	return map[string]bool{}
}

func (plugin *AtomPlugin) Init() error {
	return nil
}

type AtomPost struct {
	Timestamp *time.Time
	ID        string
	Title     string
	Link      string
}

type ByTimestamp []AtomPost

func (posts ByTimestamp) Len() int {
	return len(posts)
}

func (posts ByTimestamp) Less(i, j int) bool {
	if posts[i].Timestamp == nil {
		return true
	}

	if posts[j].Timestamp == nil {
		return false
	}

	return posts[i].Timestamp.Before(*posts[j].Timestamp)
}

func (posts ByTimestamp) Swap(i, j int) {
	posts[i], posts[j] = posts[j], posts[i]
}

func (plugin *AtomPlugin) Check(offset interface{}, context PluginContext) (interface{}, error) {
	context.Info.Printf("Checking Atom feed at %s for updates...", plugin.FeedURL)

	res, err := context.HTTP.Get(plugin.FeedURL)
	if err != nil {
		return offset, fmt.Errorf("could not read Atom feed at '%s': %w", plugin.FeedURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		logLevel := context.Info
		if offset == nil {
			logLevel = context.Error
		}
		logLevel.Printf("Could not find Atom feed at '%s'. Site might be down.", plugin.FeedURL)
		return offset, nil
	}

	fp := atom.Parser{}
	atomFeed, err := fp.Parse(res.Body)
	if err != nil {
		return offset, err
	}

	if len(atomFeed.Entries) == 0 {
		context.Info.Printf("No entries in Atoom feed at '%s'.", plugin.FeedURL)
		return offset, nil
	}

	handledEntries := make(map[string]bool)
	if offset != nil {
		handledEntries = offset.(map[string]bool)
	}

	var sortedEntries []AtomPost

	for _, entry := range atomFeed.Entries {
		if handled, present := handledEntries[entry.ID]; present && handled {
			continue
		}

		link := entry.Links[0].Href
		hasExcludedTag, err := plugin.HasExcludedTag(link, context.HTTP)
		if err != nil {
			return offset, fmt.Errorf("could not fully handle Atom feed at '%s': %w", plugin.FeedURL, err)
		}

		if hasExcludedTag {
			handledEntries[entry.ID] = true
			context.Info.Printf("Skipping post '%s' from feed at '%s' as it has an excluded tag", entry.Title, plugin.FeedURL)
			continue
		}

		sortedEntries = append([]AtomPost{{
			Timestamp: entry.PublishedParsed,
			ID:        entry.ID,
			Title:     entry.Title,
			Link:      link,
		}}, sortedEntries...)
	}

	sort.Sort(ByTimestamp(sortedEntries))

	if len(sortedEntries) == 0 {
		context.Info.Printf("No posts to report from Atom feed at '%s'.", plugin.FeedURL)
		return offset, nil
	}

	context.Info.Printf("Reporting posts from Atom feed at '%s'...", plugin.FeedURL)

	if len(plugin.Nickname) == 0 {
		plugin.Nickname = atomFeed.Title
		context.Info.Printf(
			"No nickname was provided for Atom feed at '%s', using feed title '%s' as fallback nickname",
			plugin.FeedURL,
			plugin.Nickname,
		)
	}

	if len(plugin.Message) == 0 {
		plugin.Message = "A new blog post was published"
		context.Info.Printf(
			"No message was provided for Atom feed at '%s', using default",
			plugin.FeedURL,
		)
	}

	for _, entry := range sortedEntries {
		var entryAge time.Duration
		if entry.Timestamp != nil {
			entryAge = time.Since(*entry.Timestamp)
		}

		if plugin.MinAge != nil && entryAge < *plugin.MinAge {
			context.Info.Printf("Skipping post '%s' from feed at '%s' for now as it is too new", entry.Title, plugin.FeedURL)
			continue
		}

		if plugin.MaxAge != nil && entryAge > *plugin.MaxAge {
			handledEntries[entry.ID] = true
			context.Info.Printf("Skipping post '%s' from feed at '%s' as it is too old", entry.Title, plugin.FeedURL)
			continue
		}

		if err = context.Discord.SendWithCustomAvatar(
			fmt.Sprintf("%s\n%s", plugin.Message, entry.Link),
			plugin.Nickname,
			plugin.AvatarURL,
			nil,
		); err != nil {
			return handledEntries, err
		}

		handledEntries[entry.ID] = true

		context.Info.Printf("Reported post '%s' from feed at '%s'", entry.Title, plugin.FeedURL)
	}

	return handledEntries, nil
}

func (plugin *AtomPlugin) HasExcludedTag(link string, httpClient HTTPClient) (bool, error) {
	if len(plugin.ExcludedTags) == 0 {
		return false, nil
	}

	res, err := httpClient.Get(link)

	if err != nil {
		return false, fmt.Errorf("could not read entry '%s': %w", link, err)
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return false, err
	}

	tags := doc.Find(".article__meta-tags .tags a.button")

	if tags.Length() == 0 {
		return false, nil
	}

	excludedTagFound := false
	tags.EachWithBreak(func(i int, tag *goquery.Selection) bool {
		if slices.Contains(plugin.ExcludedTags, tag.Text()) {
			excludedTagFound = true
			return false
		}

		return true
	})

	return excludedTagFound, nil
}
