package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/polymr/polymr/internal/agents"
	"github.com/polymr/polymr/internal/sanitize"
	slackapi "github.com/slack-go/slack"
)

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

			// Run the agent and tie the session to this thread
			result, err := manager.SpawnAgentInThread(command, text, channelID, threadTS)
			if err != nil {
				result = fmt.Sprintf("Agent error: %v", err)
			}

			// Update the thread message with the real result
			Client.PostMessage(channelID,
				slackapi.MsgOptionText(result, false),
				slackapi.MsgOptionTS(threadTS),
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

		threadTS, _ := event["thread_ts"].(string)

		switch eventType {
		case "app_mention":
			go handleMention(manager, text, channel, threadTS)
		case "message":
			if threadTS != "" {
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
		result, found, err := manager.ContinueThread(threadTS, prompt)
		if err != nil {
			PostThreadReply(channel, threadTS, fmt.Sprintf("Error: %v", err))
			return
		}
		if found {
			PostThreadReply(channel, threadTS, result)
			return
		}
		// No session for this thread — fall through to start a new one.
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
		if threadTS != "" {
			PostThreadReply(channel, threadTS, fmt.Sprintf("Agent error: %v", err))
		} else {
			PostMessage(channel, fmt.Sprintf("Agent error: %v", err))
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
