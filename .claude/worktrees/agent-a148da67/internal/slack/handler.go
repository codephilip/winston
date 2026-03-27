package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

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
		responseURL := r.FormValue("response_url")
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

		// Acknowledge immediately (Slack requires response within 3s).
		// Empty 200 — the async goroutine posts the real "thinking..." message.
		w.WriteHeader(http.StatusOK)

		// Spawn agent asynchronously, create a thread, store session
		go func() {
			// Post the initial "thinking" message to get a thread TS
			_, threadTS, err := Client.PostMessage(channelID,
				slackapi.MsgOptionText(
					fmt.Sprintf("*/%s* › %s\n_thinking..._", command, text), false,
				),
				slackapi.MsgOptionUsername(BotDisplayName),
			)
			if err != nil {
				postAsyncResponse(responseURL, channelID, command, fmt.Sprintf("Failed to post: %v", err))
				return
			}

			// Sanitize input before passing to agent
			text = sanitize.Input(text)

			// Set up streaming updater that edits the "thinking..." message in-place
			updater := NewStreamingSlackUpdater(channelID, threadTS)

			// Run the agent with streaming and tie the session to this thread
			result, err := manager.SpawnAgentInThreadStreaming(command, text, channelID, threadTS, updater.Update)
			if err != nil {
				result = fmt.Sprintf("Agent error: %v", err)
			}

			// Final update: replace the message with the complete result
			finalText := result
			if len(finalText) > 3000 {
				finalText = finalText[:2950] + "\n\n_...response truncated_"
			}
			Client.UpdateMessage(channelID,
				threadTS,
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
		text, _ := event["text"].(string)
		channel, _ := event["channel"].(string)

		switch eventType {
		case "app_mention":
			go handleMention(manager, text, channel)
		case "message":
			if threadTS, ok := event["thread_ts"].(string); ok {
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
func handleMention(manager *agents.Manager, text, channel string) {
	// Strip the bot mention prefix
	// Text comes in as "<@BOTID> do something" — extract the command
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		PostMessage(channel, "Mention me with a command, e.g. `@Winston /marketing analyze our latest campaign`")
		return
	}

	prompt := parts[1]

	// Check if the message starts with an agent name
	agentName, agentPrompt := parseAgentFromText(prompt, manager.AgentNames())
	if agentName == "" {
		// Build agent list dynamically
		lines := "Which agent should I use?\n"
		for _, name := range manager.AgentNames() {
			lines += fmt.Sprintf("`/%s`\n", name)
		}
		PostMessage(channel, lines)
		return
	}

	result, err := manager.SpawnAgent(agentName, sanitize.Input(agentPrompt))
	if err != nil {
		PostMessage(channel, fmt.Sprintf("Agent error: %v", err))
		return
	}

	PostMessage(channel, fmt.Sprintf("*/%s:*\n%s", agentName, result))
}

// handleThreadMessage continues a conversation in an existing thread.
func handleThreadMessage(manager *agents.Manager, text, channel, threadTS string) {
	// Try to resume the existing Claude session for this thread
	result, found, err := manager.ContinueThread(threadTS, sanitize.Input(text))
	if err != nil {
		PostThreadReply(channel, threadTS, fmt.Sprintf("Error: %v", err))
		return
	}
	if !found {
		// No active session for this thread — ignore (not a Winston thread)
		return
	}

	PostThreadReply(channel, threadTS, result)
}

// handleTopicSelection processes when a user clicks a YouTube topic button.
func handleTopicSelection(manager *agents.Manager, topicValue, channel string) {
	prompt := fmt.Sprintf("The user selected topic: %q. Generate a full video script for this topic, including hooks, segments, and CTAs. Then generate a thumbnail using Nano Banana.", topicValue)

	result, err := manager.SpawnAgent("youtube", sanitize.Input(prompt))
	if err != nil {
		PostMessage(channel, fmt.Sprintf("Error generating script: %v", err))
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
		PostMessage(channel, fmt.Sprintf("Error: %v", err))
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
