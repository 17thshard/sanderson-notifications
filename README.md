Brandon Sanderson Notifications
===============================

This is a simple Go application generating Discord messages from the media channels by fantasy author Brandon Sanderson.

Currently the following channels trigger a notification:
 * [Twitter](https://twitter.com/BrandSanderson)
 * Progress Updates on the [author's website](https://brandonsanderson.com)

Executing the application performs a single round of checks for updates, so it must be used from
an external task scheduler - such as `cron` - for periodic checks.

The following environment variables must be set for the application to run as intended:
 * `DISCORD_WEBHOOK`: The URL of the Discord webhook the updates should be dispatched to
 * `TWITTER_TOKEN`: A Twitter API OAuth2 app token, used for retrieving the most recent tweets

Furthermore, the executing user must have write access to the working directory.