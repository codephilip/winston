package slack

import (
	"fmt"
	"os"
	"strings"

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

// resolveChannel converts a channel name (with or without "#") to a channel ID.
// If the input already looks like a channel ID (starts with "C" or "G"), it is returned as-is.
func resolveChannel(channel string) (string, error) {
	channel = strings.TrimPrefix(channel, "#")
	if strings.HasPrefix(channel, "C") || strings.HasPrefix(channel, "G") {
		return channel, nil
	}
	// Look up by name
	cursor := ""
	for {
		params := &slackapi.GetConversationsParameters{
			Types:           []string{"public_channel", "private_channel"},
			Limit:           200,
			Cursor:          cursor,
			ExcludeArchived: true,
		}
		channels, nextCursor, err := Client.GetConversations(params)
		if err != nil {
			return "", fmt.Errorf("listing conversations: %w", err)
		}
		for _, ch := range channels {
			if ch.Name == channel {
				return ch.ID, nil
			}
		}
		if nextCursor == "" {
			return "", fmt.Errorf("channel %q not found", channel)
		}
		cursor = nextCursor
	}
}

// ensureInChannel joins the channel if needed and returns the resolved channel ID.
// Slack's conversations.join is idempotent — joining a channel the bot is already in is a no-op.
func ensureInChannel(channel string) (string, error) {
	id, err := resolveChannel(channel)
	if err != nil {
		return "", err
	}
	_, _, _, err = Client.JoinConversation(id)
	if err != nil && !strings.Contains(err.Error(), "already_in_channel") {
		return id, fmt.Errorf("joining channel: %w", err)
	}
	return id, nil
}

// PostMessage sends a message to a Slack channel, auto-joining if needed.
func PostMessage(channel, text string) error {
	id, err := ensureInChannel(channel)
	if err != nil {
		return fmt.Errorf("ensure in channel: %w", err)
	}
	_, _, err = Client.PostMessage(id,
		slackapi.MsgOptionText(text, false),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	return err
}

// PostThreadReply sends a threaded reply in a Slack channel, auto-joining if needed.
func PostThreadReply(channel, threadTS, text string) error {
	id, err := ensureInChannel(channel)
	if err != nil {
		return fmt.Errorf("ensure in channel: %w", err)
	}
	_, _, err = Client.PostMessage(id,
		slackapi.MsgOptionText(text, false),
		slackapi.MsgOptionTS(threadTS),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	return err
}

// PostBlocks sends a Block Kit message to a Slack channel, auto-joining if needed.
func PostBlocks(channel string, blocks ...slackapi.Block) error {
	id, err := ensureInChannel(channel)
	if err != nil {
		return fmt.Errorf("ensure in channel: %w", err)
	}
	_, _, err = Client.PostMessage(id,
		slackapi.MsgOptionBlocks(blocks...),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	return err
}
