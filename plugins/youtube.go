package plugins

import (
	"fmt"
	"github.com/mmcdole/gofeed/atom"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"net/http"
	"net/url"
	"time"
)

type YouTubePlugin struct {
	ChannelId         string `mapstructure:"channelId"`
	Nickname          string
	Messages          map[string]string
	Token             string
	ExcludedPostTypes []string `mapstructure:"excludedPostTypes"`

	excludedTypes map[string]bool
	client        *http.Client
}

func (plugin *YouTubePlugin) Name() string {
	return "youtube"
}

func (plugin *YouTubePlugin) Validate() error {
	if len(plugin.ChannelId) == 0 {
		return fmt.Errorf("channel ID for YouTube must not be empty")
	}

	if len(plugin.Token) == 0 {
		return fmt.Errorf("API token for YouTube must not be empty")
	}

	plugin.excludedTypes = make(map[string]bool)
	for _, postType := range plugin.ExcludedPostTypes {
		plugin.excludedTypes[postType] = true
	}

	return nil
}

func (plugin *YouTubePlugin) OffsetPrototype() interface{} {
	return map[string]bool{}
}

type YouTubePost struct {
	ID      string
	Title   string
	Link    string
	VideoID string
}

func (plugin *YouTubePlugin) Check(offset interface{}, context PluginContext) (interface{}, error) {
	context.Info.Println("Checking for YouTube updates...")

	plugin.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	res, err := plugin.client.Get(fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", url.QueryEscape(plugin.ChannelId)))
	if err != nil {
		return offset, fmt.Errorf("could not read YouTube feed for channel '%s': %w", plugin.ChannelId, err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		logLevel := context.Info
		if offset == nil {
			logLevel = context.Error
		}
		logLevel.Printf("Could not find feed for channel ID '%s'. YouTube API might be down.", plugin.ChannelId)
		return offset, nil
	}

	fp := atom.Parser{}
	atomFeed, err := fp.Parse(res.Body)
	if err != nil {
		return offset, err
	}

	if len(atomFeed.Entries) == 0 {
		context.Info.Println("No entries in YouTube feed.")
		return offset, nil
	}

	youtubeService, err := youtube.NewService(*context.Context, option.WithAPIKey(plugin.Token))

	handledEntries := make(map[string]bool)
	if offset != nil {
		handledEntries = offset.(map[string]bool)
	}

	var sortedEntries []YouTubePost

	for _, entry := range atomFeed.Entries {
		if handled, present := handledEntries[entry.ID]; present && handled {
			continue
		}

		videoId := ""
		if len(entry.Extensions["yt"]["videoId"]) > 0 {
			videoId = entry.Extensions["yt"]["videoId"][0].Value
		}

		sortedEntries = append([]YouTubePost{{
			ID:      entry.ID,
			Title:   entry.Title,
			Link:    entry.Links[0].Href,
			VideoID: videoId,
		}}, sortedEntries...)
	}

	if len(sortedEntries) == 0 {
		context.Info.Println("No YouTube posts to report.")
		return offset, nil
	}

	context.Info.Println("Reporting YouTube posts...")

	if len(plugin.Nickname) == 0 && len(plugin.Messages) == 0 {
		plugin.Nickname = atomFeed.Title
		context.Info.Printf(
			"No nickname or specific messages were provided for channel '%s', using feed title '%s' as fallback nickname",
			plugin.ChannelId,
			plugin.Nickname,
		)
	}

	for _, entry := range sortedEntries {
		info, err := plugin.buildPostInfo(entry, youtubeService)
		if err != nil {
			return handledEntries, err
		}

		if exclude, present := plugin.excludedTypes[info.Type]; present && exclude {
			context.Info.Printf("Ignoring YouTube %s '%s'", info.Type, entry.Title)
			handledEntries[entry.ID] = true

			continue
		}

		template := info.DefaultTemplate
		if configTemplate, exists := plugin.Messages[info.Type]; exists {
			template = configTemplate
		}

		message := template
		if info.FormatMessage != nil {
			message = info.FormatMessage(template)
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

		context.Info.Printf("Reported YouTube post '%s'", entry.Title)
	}

	return handledEntries, nil
}

type postInfo struct {
	Type            string
	DefaultTemplate string
	FormatMessage   func(string) string
}

func (plugin *YouTubePlugin) buildPostInfo(entry YouTubePost, youtubeService *youtube.Service) (*postInfo, error) {
	if entry.VideoID == "" {
		return nil, nil
	}

	info, err := plugin.buildLiveEventInfo(entry, youtubeService)
	if info != nil || err != nil {
		return info, err
	}

	info, err = plugin.buildShortInfo(entry)
	if info != nil || err != nil {
		return info, err
	}

	info = &postInfo{
		Type:            "video",
		DefaultTemplate: fmt.Sprintf("%s posted something on YouTube", plugin.Nickname),
	}

	return info, nil
}

func (plugin *YouTubePlugin) buildLiveEventInfo(entry YouTubePost, youtubeService *youtube.Service) (*postInfo, error) {
	videoList, err := youtubeService.Videos.List([]string{"liveStreamingDetails", "status"}).Id(entry.VideoID).Do()

	if err != nil {
		return nil, err
	}

	if len(videoList.Items) == 0 {
		return nil, nil
	}

	video := videoList.Items[0]
	if video.LiveStreamingDetails == nil {
		return nil, nil
	}

	parsedStart, err := time.Parse(time.RFC3339, video.LiveStreamingDetails.ScheduledStartTime)
	if err != nil {
		return nil, nil
	}

	// Treat past live events as regular videos
	if parsedStart.Before(time.Now()) {
		return nil, nil
	}

	info := postInfo{
		Type:            "livestream",
		DefaultTemplate: fmt.Sprintf("%s is going live on YouTube %%s!", plugin.Nickname),
		FormatMessage: func(template string) string {
			return fmt.Sprintf(template, fmt.Sprintf("<t:%d:R>", parsedStart.Unix()))
		},
	}

	if video.Status.UploadStatus == "processed" {
		info.Type = "premiere"
		info.DefaultTemplate = fmt.Sprintf("%s will premiere a video on YouTube %%s!", plugin.Nickname)
	}

	return &info, nil
}

func (plugin *YouTubePlugin) buildShortInfo(entry YouTubePost) (*postInfo, error) {
	response, err := plugin.client.Head(fmt.Sprintf("https://www.youtube.com/shorts/%s", entry.VideoID))

	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, nil
	}

	info := postInfo{
		Type:            "short",
		DefaultTemplate: fmt.Sprintf("%s posted a short on YouTube!", plugin.Nickname),
	}

	return &info, nil
}
