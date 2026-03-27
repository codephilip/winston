package slack

import (
	"os"

	slackapi "github.com/slack-go/slack"
)

// BotDisplayName is the name shown on all bot messages in Slack.
const BotDisplayName = "Winston"

// Client wraps the Slack API client as a singleton.
var Client *slackapi.Client

func init() {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token != "" {
		Client = slackapi.New(token)
	}
}

// PostMessage sends a message to a Slack channel.
func PostMessage(channel, text string) error {
	_, _, err := Client.PostMessage(channel,
		slackapi.MsgOptionText(text, false),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	return err
}

// PostThreadReply sends a threaded reply in a Slack channel.
func PostThreadReply(channel, threadTS, text string) error {
	_, _, err := Client.PostMessage(channel,
		slackapi.MsgOptionText(text, false),
		slackapi.MsgOptionTS(threadTS),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	return err
}

// PostBlocks sends a Block Kit message to a Slack channel.
func PostBlocks(channel string, blocks ...slackapi.Block) error {
	_, _, err := Client.PostMessage(channel,
		slackapi.MsgOptionBlocks(blocks...),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	return err
}
