Brandon Sanderson Notifications
===============================

This is a simple Go application to generate Discord messages from various media channels. Currently, this is mainly used
for fantasy author Brandon Sanderson's channels.

The following channels can trigger a notification:

 * [Twitter](https://twitter.com/)
 * Progress Updates on an author's website (e.g. [Brandon Sanderson](https://brandonsanderson.com))
 * [YouTube](https://www.youtube.com/) videos and livestreams

Executing the application performs a single round of checks for updates, so it must be used from an external task
scheduler - such as `cron` - for periodic checks.

## Usage
This application uses a simple plugin system that can be configured with a YAML file. We distinguish "plugins" from "
connectors". *Plugins* provide the support for checking social media for updates, while *connectors* are instances of
those plugins that reflect a single channel on the respective platform. This means that there can be many connectors for
different channels on the same platform that all use the same plugin.

All connectors send their updates to the same Discord channel.

The basic structure of a config file is as follows

```yaml
discordWebhook: '<webhook-id>'
discordMentions:
  roles: ['<role-id>']
  users: ['<user-id>']
shared:
  progress:
    url: https://brandon-sanderson.com
connectors:
  connector1:
    plugin: progress
    config:
      url: https://brandonsanderson.com
      message: The progress bars on Brandon's website were updated!
  connector2:
    plugin: twitter
    config:
      account: BrandSanderson
```

The `discordWebhook` item is mandatory and must be the ID (i.e. channel ID + token) of a Discord webhook. Simply use the
value after `https://discord.com/api/webhooks/` from the webhook URL Discord provides you with.

The `discordMentions` item can optionally be specified to have all webhook messages contain mentions for the listed roles
and users. _Note that in general no additional mentions will be parsed from messages, including `@everyone`._

The `shared` section defines configuration values that are used across all connectors using a plugin. Keys in the map
must be a plugin ID. The shared config object is simply merged into any connector-specific one. Connector configs always
take precedence over shared ones.

The `connectors` section defines the actual connectors that will be used to check for updates. Each key serves as unique
identifier to keep track of the status of the channel the connector consumes. You must specify a `plugin` for the connector.
The `config` value is optional and may contain plugin-specific options.

See the respective [plugin sections](#plugins) for which plugins and options are available in the `shared` and
connector-level sections.

Once you have set up your config file, simply invoke the following command:
```shell
sanderson-notifications [-config config.yaml] [-offsets offsets.json]
```
The `-config` and `-offsets` options are not mandatory and shown here with their default values.
Respectively, they point to the config file to load as well as the location where to retrieve and store connector offsets.

Furthermore, the executing user must have write access to the working directory.

## Offsets
A connector always has an associated "offset", which the underlying plugin may use to keep track of
what on a social media channel it has seen last. After every connector run, a new offset for it is stored.

Offsets are stored in a JSON file that contains a simple JSON object. Keys are connector names,
while values are plugin-specific JSON values that contain the current offset for a connector.

Offsets for unknown connectors are retained, in case they were only temporarily removed from the configuration file.

A sample offset file may look like this:
```json
{
  "unknown-connector": {
    "myCustomJson": "value"
  },
  "progress-connector": [
    {
      "Title": "Progress Bar 1",
      "Link": "",
      "Value": 100
    },
    {
      "Title": "Progress Bar 2",
      "Link": "https://example.com",
      "Value": 61
    }
  ],
  "twitter-connector": "1439074304365264899",
  "youtube-connector": {
    "yt:video:--sqRKutFMI": true,
    "yt:video:-Z4_2gYl_ug": true,
    "yt:video:-hO7fM9EHU4": true
  }
}
```

The offsets file may be manually edited.

## Plugins
The plugins are listed with their IDs in parentheses. Besides available configuration options and the offset storage format,
the change detection mechanism is also explained.

### Atom Feed (`atom`)
Checks an [Atom feed](https://datatracker.ietf.org/doc/html/rfc4287) (see e.g. [The Cognitive Realm Blog](https://www.dragonsteelbooks.com/blogs/the-cognitive-realm.atom))
for new entries. If no starting offset is specified, all entries currently in the feed will be posted.

#### Configuration
The YAML structure for this plugin's configuration is as follows:
```yaml
feedUrl: https://www.dragonsteelbooks.com/blogs/the-cognitive-realm.atom
nickname: The Cognitive Realm Blog
avatarUrl: https://raw.githubusercontent.com/Palanaeum/sanderson-notifications/master/avatars/dragonsteel.png
message: A new blog post was published to The Cognitive Realm!
excludedTags: [Weekly Update]
minAge: 5m
maxAge: 168h
```
| Field          | Mandatory | Description                                                                                                                                                                                                                                                                |
|----------------|:---------:|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `feedUrl`      |    ✔️     | URL of the Atom feed                                                                                                                                                                                                                                                       |
| `nickname`     |     ❌     | Nickname to use for the webhook Discord message. Will use the feed title by default                                                                                                                                                                                        |
| `avatarUrl`    |     ❌     | URL of an avatar to use for the webhook Discord message. Will use the avatar configured for the webhook globally by default                                                                                                                                                |
| `message`      |     ❌     | Message to display preceding the link to an entry                                                                                                                                                                                                                          |
| `excludedTags` |     ❌     | List of tags that must not be present on a blog post to be included. If *any* of these tags is present, the post will be excluded. **Note:** Tags load the URL of the post and assume Dragonsteel's tagging format                                                         |
| `minAge`       |     ❌     | The minimum age a blog post needs to have (from its publishing date) to be included. Can be used to prevent unintentionally or prematurely published articles from being posted. Accepts any value that [Go's `ParseDuration`](https://pkg.go.dev/time#ParseDuration) does |
| `maxAge`       |     ❌     | The maximum age a blog post may have (from its publishing date) to be included. Accepts any value that [Go's `ParseDuration`](https://pkg.go.dev/time#ParseDuration) does                                                                                                  |

#### Offset format
Offsets are stored as a JSON object such as
```json
{
  "https://www.dragonsteelbooks.com/blogs/the-cognitive-realm/light-day-2024": true,
  "https://www.dragonsteelbooks.com/blogs/the-cognitive-realm/adapting-stonewalkers": true,
  "https://www.dragonsteelbooks.com/blogs/the-cognitive-realm/brandon-sanderson-fanx24": true
}
```
Keys are feed entry IDs and values indicate whether the entry has been processed.
Offsets as stored by the application will always have `true` as value, but you may manually change an entry to `false`.

In this case, the entry will be posted to Discord again if it's still in the feed.

#### Change detection
The current content of the Atom feed is retrieved. Feed entries that are marked with `true` in the current offset are omitted.

If this list of feed entries is non-empty after this process, links to the corresponding entries will be posted in chronological order.

Note how *all* feed entries are inspected again and offsets contain many entries. This is due to the fact that an Atom feed
may put new entries in-between previously checked ones, so for correctness all of them must be inspected again.

### Author Progress (`progress`)
Checks progress bars on an author's website for changes. This plugin is only built with [Brandon Sanderson's website](https://www.brandonsanderson.com/)
in mind, so it will most likely not work for other author's progress bars, should they have them.

#### Configuration
The YAML structure for this plugin's configuration is as follows:
```yaml
url: https://brandonsanderson.com
message: The progress bars on Brandon's website were updated!
debounceDelay: 2m
```
| Field           | Mandatory | Description                                                                                         |
|-----------------|:---------:|-----------------------------------------------------------------------------------------------------|
| `url`           |    ✔️     | URL of the author's website                                                                         |
| `message`       |    ✔️     | Message to display preceding the embed with progress updates                                        |
| `debounceDelay` |     ❌     | Wait time after last change before posting (e.g. "2m", "30s"). If omitted or "0", posts immediately |

#### Offset format
Offsets are stored as a JSON object with the following structure:
```json
{
  "Published": [
    {
      "Title": "Progress Bar 1",
      "Link": "https://example.com",
      "Value": 75
    },
    {
      "Title": "Progress Bar 2", 
      "Link": "",
      "Value": 100
    }
  ],
  "Observed": [
    {
      "Title": "Progress Bar 1",
      "Link": "https://example.com", 
      "Value": 80
    },
    {
      "Title": "Progress Bar 2",
      "Link": "",
      "Value": 100
    }
  ],
  "DebounceStart": "2023-10-15T14:30:00Z"
}
```
 * `Published`: The progress bar state that was last posted to Discord
 * `Observed`: The current progress bar state observed from the website
 * `DebounceStart`: When the debounce period started (null when not debouncing)

When `debounceDelay` is configured, changes are detected by comparing `Published` and `Observed`. If differences exist and the debounce period has elapsed, the changes are posted to Discord and `Observed` becomes the new `Published` state.

#### Change detection
Progress bars are scraped from the website HTML and compared with the stored `Published` state:
 * Progress bars that exist on the website but not in `Published` are marked as *new*
 * Progress bars that exist in both but have different progress values are marked as *changed*
 * Progress bars that exist in both with the same progress values are *retained*
 * Progress bars that exist in `Published` but not on the website are *ignored*

If any *new* or *changed* progress bars are detected:
 * With `debounceDelay = 0`: Changes are posted immediately
 * With `debounceDelay > 0`: A debounce timer starts/resets, and changes are posted only after the delay period with no further changes

### Twitter Timeline (`twitter`)
Checks a Twitter account's timeline for new tweets. This *includes* retweets, but *omits* replies.

**Note:** You *must* specify a starting offset in the offset file for this plugin to work.
Use the ID of the tweet immediately *before* the first one you want to be posted.
If you want all tweets for an account to be posted, simply use the first tweet's ID minus 1.

#### Configuration
The YAML structure for this plugin's configuration is as follows:
```yaml
account: BrandSanderson
nickname: Brandon
tweetMessage: Brandon tweeted
retweetMessage: Brandon retweeted
excludeRetweetsOf: ['DragonsteelBook']

loginUser: dummy_user
loginPassword: foobar
cookiePath: twitter-cookies.json
```
| Field               | Mandatory | Description                                                                               |
|---------------------|:---------:|-------------------------------------------------------------------------------------------|
| `account`           |     ✔️    | Twitter handle (without `@`) for account to check tweets for                              |
| `nickname`          |     ❌    | Nickname for the Twitter account to use in Discord messages                               |
| `tweetMessage`      |     ❌    | Custom message to display for new tweets                                                  |
| `retweetMessage`    |     ❌    | Custom message to display for new retweets                                                |
| `excludeRetweetsOf` |     ❌    | List of Twitter handles (without `@`) for which retweets should *not* be posted           |
| `loginUser`         |     ❌    | Username for logging into Twitter to access API                                           |
| `loginPassword`     |     ❌    | Password for logging into Twitter to access API                                           |
| `cookiePath`        |     ❌    | Path to writable file where cookies can be stored to not require logging in for every run |

If `nickname` and `tweetMessage` as well as `retweetMessage` are all omitted,
the Twitter display name for the account will be used in a standard message.

If no login credentials are provided, a default "open account" will be used which may not work.

#### Offset format
Offsets are stored as a JSON string such as
```json
"943172525596405761"
```
The stored value is the ID of the *last* tweet that was read from the timeline.

#### Change detection
All tweets that were posted to the timeline since the tweet corresponding to the stored offset are retrieved.
Due to Twitter's API limitations, a maximum of 3200 tweets will be retrieved.

Any tweet and retweet that has been posted since the offset and is not from an excluded account will be posted in chronological order.

### YouTube Feed (`youtube`)
Checks a YouTube channel's atom feed (see e.g. [Brandon Sanderson's channel](https://www.youtube.com/feeds/videos.xml?channel_id=UC3g-w83Cb5pEAu5UmRrge-A))
for new videos and livestreams. If no starting offset is specified, all videos currently in the feed will be posted.

Note that this requires access to the YouTube API for identifying livestreams and related data like scheduled start times, to this end, you need to acquire an API token for the YouTube Data API.

#### Configuration
The YAML structure for this plugin's configuration is as follows:
```yaml
channelId: ChannelId
token: youtubeToken
nickname: Brandon
messages:
  video: Brandon posted a video on YouTube
  livestream: Brandon will be streaming live %s
excludedPostTypes:
  - short
```
| Field               | Mandatory | Description                                                                                  |
|---------------------|:---------:|----------------------------------------------------------------------------------------------|
| `channelId`         |     ✔️     | The *ID* of the YouTube channel for which to check the feed                                  |
| `token`             |     ✔️     | Token for the YouTube Data API v3                                                            |
| `nickname`          |     ❌    | Nickname for the YouTube channel to use in Discord messages                                  |
| `messages`          |     ❌    | A dictionary where keys represent the post type and values are custom messages for that type |
| `excludedPostTypes` |     ❌    | A list of post types from the feed not to report                                             |

Note that the *ID* of the channel is required here, which can differ from the username visible in a channel's URL.
A channel ID can be retrieved from a channel page's source code.

If `nickname` and `messages` are all omitted, the channel name for the YouTube channel will be used in a standard message.

Both `messages` and `excludedPostTypes` support several different post types, namely `short`, `livestream`, `premiere`, and `video`.
The latter is used by default if no other type could be identified.
The messages for `livestream` and `premiere` can use `%s` within their definition as a placeholder for a relative timestamp in the Discord message.

#### Acquiring an API token
Getting access to the YouTube Data API, like most other Google services, requires a Google Cloud project.
See the [official guide](https://developers.google.com/workspace/guides/create-project) for setting that up.

Within your project's Cloud Console, you must enable the [YouTube Data API v3](https://console.cloud.google.com/apis/api/youtube.googleapis.com).
Then you can create [API key credentials](https://console.cloud.google.com/apis/api/youtube.googleapis.com/credentials) for that API, which will be the token you need to specify in the config.

#### Offset format
Offsets are stored as a JSON object such as
```json
{
  "yt:video:--sqRKutFMI": true,
  "yt:video:-Z4_2gYl_ug": true,
  "yt:video:-hO7fM9EHU4": true,
  "yt:video:-w5f8-Elfqo": true,
  "yt:video:0cf-qdZ7GbA": true
}
```
Keys are feed entry IDs (e.g. `yt:video:<video-id>` for videos) and values indicate whether the entry has been processed.
Offsets as stored by the application will always have `true` as value, but you may manually change an entry to `false`.

In this case, the video or livestream will be posted to Discord again if it's still in the feed.

#### Change detection
The current content of the Atom feed is retrieved. Feed entries that are marked with `true` in the current offset are omitted.

If this list of feed entries is non-empty after this process, links to the corresponding videos or livestreams
will be posted in chronological order.

Note how *all* feed entries are inspected again and offsets contain many entries. This is due to the fact that YouTube's
Atom feed may put new videos in-between previously checked ones, so for correctness all of them must be inspected again.

## Current Configuration for 17th Shard Discord
The [17th Shard](https://17thshard.com) has set up a channel on their [Discord server](https://discord.gg/17thshard)
where several updates from Brandon Sanderson are automatically posted with this application.

Under the current configuration, these channels are currently checked for updates:

 * [Sanderson's Twitter account](https://twitter.com/BrandSanderson)
 * [Dragonsteel Books Twitter account](https://twitter.com/DragonsteelBook)
 * Progress Updates on the [Sandersons's website](https://brandonsanderson.com)
 * [Sanderson's YouTube channel](https://www.youtube.com/channel/UC3g-w83Cb5pEAu5UmRrge-A)

If you want to replicate this on your own server, either follow the `#brandon-updates` channel or set up this
application with the following configuration, adjusted for your server:

```yaml
discordWebhook: '<webhook-id>'
shared:
  twitter:
    token: '<twitter-auth-token>'
connectors:
  brandon-progress:
    plugin: progress
    config:
      url: https://brandonsanderson.com
      message: The progress bars on Brandon's website were updated!
  brandon-twitter:
    plugin: twitter
    config:
      account: BrandSanderson
      nickname: Brandon
      excludeRetweetsOf: [ 'DragonsteelBook' ]
  dragonsteel-books-twitter:
    plugin: twitter
    config:
      account: DragonsteelBook
      excludeRetweetsOf: [ 'BrandSanderson' ]
  brandon-youtube:
    plugin: youtube
    config:
      channelId: UC3g-w83Cb5pEAu5UmRrge-A
      nickname: Brandon
```
