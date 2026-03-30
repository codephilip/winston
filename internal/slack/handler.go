package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/polymr/polymr/internal/agents"
	"github.com/polymr/polymr/internal/sanitize"
	slackapi "github.com/slack-go/slack"
)

// StreamingSlackUpdater edits a Slack message in-place with partial agent output.
type StreamingSlackUpdater struct {
	channelID string
	threadTS  string // the timestamp of the message to update
	mu        sync.Mutex
}

// NewStreamingSlackUpdater creates a new updater for the given message.
func NewStreamingSlackUpdater(channelID, messageTS string) *StreamingSlackUpdater {
	return &StreamingSlackUpdater{
		channelID: channelID,
		threadTS:  messageTS,
	}
}

// Update implements the StreamCallback — edits the Slack message with partial output.
func (s *StreamingSlackUpdater) Update(partial string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Truncate to stay within Slack message limits
	display := partial
	if len(display) > 3000 {
		display = display[len(display)-2950:] // show the tail
		display = "_...output truncated..._\n" + display
	}

	if display == "" {
		display = "_streaming..._"
	}

	_, _, _, err := Client.UpdateMessage(
		s.channelID,
		s.threadTS,
		slackapi.MsgOptionText(display, false),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	if err != nil {
		log.Printf("[slack] streaming update failed: %v", err)
	}
}

// HandleSlashCommand processes /marketing, /pentester, /youtube commands from Slack.
func HandleSlashCommand(manager *agents.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		command := strings.TrimPrefix(r.FormValue("command"), "/")
		text := r.FormValue("text")
		channelID := r.FormValue("channel_id")
		// Validate the agent exists
		if !manager.HasAgent(command) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"response_type": "ephemeral",
				"text":          fmt.Sprintf("Unknown agent: /%s", command),
			})
			return
		}

		// Respond in_channel so Slack shows the slash command as the user's message
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"response_type": "in_channel",
			"text":          text,
		})

		// Spawn agent asynchronously — find the user's message and reply in its thread
		go func() {
			// Small delay to let Slack process the in_channel response
			time.Sleep(2 * time.Second)

			// Find the slash command response — it's the most recent message in the channel
			history, err := Client.GetConversationHistory(&slackapi.GetConversationHistoryParameters{
				ChannelID: channelID,
				Limit:     1,
			})
			if err != nil || len(history.Messages) == 0 {
				log.Printf("[slack] failed to find slash command message: %v", err)
				return
			}
			threadTS := history.Messages[0].Timestamp

			// Post streaming reply in thread
			_, replyTS, err := Client.PostMessage(channelID,
				slackapi.MsgOptionText("_thinking..._", false),
				slackapi.MsgOptionTS(threadTS),
				slackapi.MsgOptionUsername(BotDisplayName),
			)
			if err != nil {
				log.Printf("[slack] failed to post thread reply: %v", err)
				return
			}

			sanitizedText := sanitize.Input(text)
			updater := NewStreamingSlackUpdater(channelID, replyTS)

			result, err := manager.SpawnAgentInThreadStreaming(command, sanitizedText, channelID, threadTS, updater.Update)
			if err != nil {
				result = formatAgentError(command, err)
			}

			finalText := result
			if len(finalText) > 3000 {
				finalText = finalText[:2950] + "\n\n_...response truncated_"
			}
			Client.UpdateMessage(channelID,
				replyTS,
				slackapi.MsgOptionText(finalText, false),
				slackapi.MsgOptionUsername(BotDisplayName),
			)
		}()
	}
}

// HandleEvent processes Slack Events API payloads (mentions, messages in threads).
func HandleEvent(manager *agents.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Handle Slack URL verification challenge
		if t, ok := payload["type"].(string); ok && t == "url_verification" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"challenge": payload["challenge"].(string),
			})
			return
		}

		// Respond 200 immediately, process async
		w.WriteHeader(http.StatusOK)

		event, ok := payload["event"].(map[string]interface{})
		if !ok {
			return
		}

		// Ignore bot messages to prevent loops
		if _, hasBot := event["bot_id"]; hasBot {
			return
		}

		eventType, _ := event["type"].(string)
		subtype, _ := event["subtype"].(string)
		text, _ := event["text"].(string)
		channel, _ := event["channel"].(string)

		threadTS, _ := event["thread_ts"].(string)

		log.Printf("[slack] event type=%s subtype=%q channel=%s thread_ts=%q text=%q",
			eventType, subtype, channel, threadTS, truncate(text, 60))

		switch eventType {
		case "app_mention":
			go handleMention(manager, text, channel, threadTS)
		case "message":
			// Only handle plain user messages — skip bot posts, edits, deletes, etc.
			if threadTS != "" && subtype == "" {
				go handleThreadMessage(manager, text, channel, threadTS)
			}
		}
	}
}

// HandleInteraction processes Slack interactive components (buttons, menus).
func HandleInteraction(manager *agents.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		payloadStr := r.FormValue("payload")
		var interaction slackapi.InteractionCallback
		if err := json.Unmarshal([]byte(payloadStr), &interaction); err != nil {
			http.Error(w, "invalid interaction payload", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)

		for _, action := range interaction.ActionCallback.BlockActions {
			switch {
			case strings.HasPrefix(action.ActionID, "youtube_topic_"):
				go handleTopicSelection(manager, action.Value, interaction.Channel.ID)
			case action.ActionID == "agent_followup":
				go handleFollowUp(manager, action.Value, interaction.Channel.ID)
			}
		}
	}
}

// postAsyncResponse sends a delayed response back to Slack via response_url.
func postAsyncResponse(responseURL, channel, agent, result string) {
	// Truncate long responses for Slack (max ~3000 chars for readability)
	if len(result) > 3000 {
		result = result[:2950] + "\n\n_...response truncated_"
	}

	payload := map[string]interface{}{
		"response_type":    "in_channel",
		"replace_original": false,
		"text":             fmt.Sprintf("*/%s result:*\n%s", agent, result),
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(responseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("failed to post async response: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("slack async response returned %d", resp.StatusCode)
	}
}

// handleMention processes @Winston mentions and routes to the appropriate agent.
// If threadTS is non-empty, the mention is inside an existing thread and the
// response is posted as a thread reply. When a session already exists for that
// thread, the conversation is resumed instead of starting fresh.
func handleMention(manager *agents.Manager, text, channel, threadTS string) {
	// Strip the bot mention prefix
	// Text comes in as "<@BOTID> do something" — extract the command
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		reply := "Mention me with a command, e.g. `@Winston /marketing analyze our latest campaign`"
		if threadTS != "" {
			PostThreadReply(channel, threadTS, reply)
		} else {
			PostMessage(channel, reply)
		}
		return
	}

	prompt := sanitize.Input(parts[1])

	// If inside a thread, try to resume an existing session first.
	if threadTS != "" {
		log.Printf("[slack] @mention in existing thread=%s, trying to continue session", threadTS)

		// Post placeholder and stream the response
		_, msgTS, err := Client.PostMessage(channel,
			slackapi.MsgOptionText("_thinking..._", false),
			slackapi.MsgOptionTS(threadTS),
			slackapi.MsgOptionUsername(BotDisplayName),
		)
		if err != nil {
			log.Printf("[slack] failed to post placeholder: %v", err)
			PostThreadReply(channel, threadTS, fmt.Sprintf("Error: %v", err))
			return
		}

		updater := NewStreamingSlackUpdater(channel, msgTS)
		result, found, err := manager.ContinueThreadStreaming(threadTS, prompt, updater.Update)
		if err != nil {
			Client.UpdateMessage(channel, msgTS,
				slackapi.MsgOptionText(formatAgentError("session", err), false),
				slackapi.MsgOptionUsername(BotDisplayName),
			)
			return
		}
		if found {
			finalText := result
			if len(finalText) > 3000 {
				finalText = finalText[:2950] + "\n\n_...response truncated_"
			}
			Client.UpdateMessage(channel, msgTS,
				slackapi.MsgOptionText(finalText, false),
				slackapi.MsgOptionUsername(BotDisplayName),
			)
			return
		}
		// No session — delete placeholder and fall through to start a new one.
		Client.DeleteMessage(channel, msgTS)
	}

	// Check if the message starts with an agent name
	agentName, agentPrompt := parseAgentFromText(prompt, manager.AgentNames())
	if agentName == "" {
		// Build agent list dynamically
		lines := "Which agent should I use?\n"
		for _, name := range manager.AgentNames() {
			lines += fmt.Sprintf("`/%s`\n", name)
		}
		if threadTS != "" {
			PostThreadReply(channel, threadTS, lines)
		} else {
			PostMessage(channel, lines)
		}
		return
	}

	result, err := manager.SpawnAgent(agentName, sanitize.Input(agentPrompt))
	if err != nil {
		errMsg := formatAgentError(agentName, err)
		if threadTS != "" {
			PostThreadReply(channel, threadTS, errMsg)
		} else {
			PostMessage(channel, errMsg)
		}
		return
	}

	response := fmt.Sprintf("*/%s:*\n%s", agentName, result)
	if threadTS != "" {
		PostThreadReply(channel, threadTS, response)
	} else {
		PostMessage(channel, response)
	}
}

// handleThreadMessage continues a conversation in an existing thread.
func handleThreadMessage(manager *agents.Manager, text, channel, threadTS string) {
	log.Printf("[slack] thread reply in %s thread=%s text=%q", channel, threadTS, truncate(text, 80))

	// Post a placeholder that we'll update with streaming output
	_, msgTS, err := Client.PostMessage(channel,
		slackapi.MsgOptionText("_thinking..._", false),
		slackapi.MsgOptionTS(threadTS),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
	if err != nil {
		log.Printf("[slack] failed to post placeholder: %v", err)
		return
	}

	updater := NewStreamingSlackUpdater(channel, msgTS)

	result, found, err := manager.ContinueThreadStreaming(threadTS, sanitize.Input(text), updater.Update)
	if err != nil {
		log.Printf("[slack] continue thread error: %v", err)
		Client.UpdateMessage(channel, msgTS,
			slackapi.MsgOptionText(formatAgentError("session", err), false),
			slackapi.MsgOptionUsername(BotDisplayName),
		)
		return
	}
	if !found {
		log.Printf("[slack] no session for thread=%s, posting no-session notice", threadTS)
		Client.UpdateMessage(channel, msgTS,
			slackapi.MsgOptionText(":information_source: _No active session for this thread — the previous run may have failed or the server was restarted. Start a new run with a slash command._", false),
			slackapi.MsgOptionUsername(BotDisplayName),
		)
		return
	}

	// Final update with complete result
	finalText := result
	if len(finalText) > 3000 {
		finalText = finalText[:2950] + "\n\n_...response truncated_"
	}
	Client.UpdateMessage(channel, msgTS,
		slackapi.MsgOptionText(finalText, false),
		slackapi.MsgOptionUsername(BotDisplayName),
	)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// formatAgentError returns a Slack-friendly error message for agent failures.
func formatAgentError(agentName string, err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "timed out after"):
		return fmt.Sprintf(":hourglass: */%s timed out.* The agent hit its time limit before finishing. You can retry with a simpler prompt or increase the timeout.", agentName)
	case strings.Contains(msg, "max turns"):
		return fmt.Sprintf(":warning: */%s hit its turn limit.* The agent used all %s conversation turns without finishing — this usually means it got stuck in a loop. Try a more specific prompt.", agentName, extractTurns(msg))
	case strings.Contains(msg, "not found"):
		return fmt.Sprintf(":x: Agent `%s` is not registered. Available agents: check `/help`.", agentName)
	case strings.Contains(msg, "signal: killed"):
		return fmt.Sprintf(":octagonal_sign: */%s was killed.* The process was terminated — likely due to memory limits or a service restart.", agentName)
	default:
		// Truncate raw errors to keep Slack tidy
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return fmt.Sprintf(":x: */%s failed:*\n```%s```", agentName, msg)
	}
}

func extractTurns(msg string) string {
	// Try to extract the number from "max turns (50)"
	if i := strings.Index(msg, "("); i >= 0 {
		if j := strings.Index(msg[i:], ")"); j >= 0 {
			return msg[i+1 : i+j]
		}
	}
	return "its"
}

// handleTopicSelection processes when a user clicks a YouTube topic button.
func handleTopicSelection(manager *agents.Manager, topicValue, channel string) {
	prompt := fmt.Sprintf("The user selected topic: %q. Generate a full video script for this topic, including hooks, segments, and CTAs. Then generate a thumbnail using Nano Banana.", topicValue)

	result, err := manager.SpawnAgent("youtube", sanitize.Input(prompt))
	if err != nil {
		PostMessage(channel, formatAgentError("youtube", err))
		return
	}

	PostMessage(channel, fmt.Sprintf("*YouTube Script for: %s*\n\n%s", topicValue, result))
}

// handleFollowUp processes follow-up action buttons.
func handleFollowUp(manager *agents.Manager, value, channel string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return
	}
	agentName, prompt := parts[0], parts[1]

	result, err := manager.SpawnAgent(agentName, sanitize.Input(prompt))
	if err != nil {
		PostMessage(channel, formatAgentError(agentName, err))
		return
	}
	PostMessage(channel, result)
}

// parseAgentFromText extracts an agent name from message text.
// Supports formats: "/marketing do X", "marketing: do X", "marketing do X"
func parseAgentFromText(text string, agentNames []string) (string, string) {
	text = strings.TrimSpace(text)

	for _, name := range agentNames {
		prefixes := []string{
			"/" + name + " ",
			name + ": ",
			name + " ",
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(strings.ToLower(text), prefix) {
				return name, strings.TrimSpace(text[len(prefix):])
			}
		}
	}

	return "", text
}
