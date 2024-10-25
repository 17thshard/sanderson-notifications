package plugins

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"net/http"
	"sort"
	"time"
)

type AtomPlugin struct {
	FeedURL   string `mapstructure:"feedUrl"`
	Nickname  string
	AvatarURL string `mapstructure:"avatarUrl"`
	Message   string

	client *http.Client
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

	plugin.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	res, err := plugin.client.Get(plugin.FeedURL)
	if err != nil {
		return offset, fmt.Errorf("could not read Atoom feed at '%s': %w", plugin.FeedURL, err)
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

		sortedEntries = append([]AtomPost{{
			Timestamp: entry.PublishedParsed,
			ID:        entry.ID,
			Title:     entry.Title,
			Link:      entry.Links[0].Href,
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
