package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/go-chi/chi/v5"
)

// StreamCallback is called periodically with accumulated output during streaming.
type StreamCallback func(partial string)

// AgentConfig defines a registered agent and its capabilities.
type AgentConfig struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Model       string        `json:"model,omitempty"`
	Timeout     time.Duration `json:"-"`
}

// agentConfigJSON is the JSON wire format for AgentConfig (timeout as integer seconds).
type agentConfigJSON struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Model          string `json:"model,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (a AgentConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(agentConfigJSON{
		Name:           a.Name,
		Description:    a.Description,
		Model:          a.Model,
		TimeoutSeconds: int(a.Timeout.Seconds()),
	})
}

// Session represents an active agent conversation tied to a Slack thread.
type Session struct {
	ClaudeSessionID string    `json:"claude_session_id"` // Claude --resume ID
	AgentID         string    `json:"agent_id"`
	SlackThreadTS   string    `json:"slack_thread_ts"`   // Slack thread timestamp (unique per thread)
	SlackChannel    string    `json:"slack_channel"`
	LastUsed        time.Time `json:"last_used"`
}

// claudeResult is the JSON output from `claude --output-format json`.
type claudeResult struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	Result    string `json:"result"`
	IsError   bool   `json:"is_error"`
	SessionID string `json:"session_id"`
}

// Schedule represents a scheduled agent run.
type Schedule struct {
	ID      string       `json:"id"`
	AgentID string       `json:"agent_id"`
	Cron    string       `json:"cron"`
	Prompt  string       `json:"prompt"`
	SlackCh string       `json:"slack_channel,omitempty"`
	Status  string       `json:"status"`
	EntryID cron.EntryID `json:"-"` // cron scheduler entry
}

// SlackPoster is a function that posts a message to a Slack channel.
type SlackPoster func(channel, text string) error

// Manager handles agent lifecycle and routing.
type Manager struct {
	mu          sync.RWMutex
	agents      map[string]*AgentConfig
	sessions    map[string]*Session // key: slackThreadTS
	schedules   map[string]*Schedule
	cron        *cron.Cron
	schedCount  int
	SlackPost   SlackPoster
}

func NewManager() *Manager {
	c := cron.New()
	c.Start()

	m := &Manager{
		agents:    make(map[string]*AgentConfig),
		sessions:  make(map[string]*Session),
		schedules: make(map[string]*Schedule),
		cron:      c,
	}

	// Load agents from ~/.claude/agents/*.md (the standard Claude Code agent directory).
	agentsDir := filepath.Join(os.Getenv("HOME"), ".claude", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		log.Printf("[agents] warning: could not read %s: %v", agentsDir, err)
	} else {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			cfg, err := parseAgentFile(filepath.Join(agentsDir, e.Name()))
			if err != nil {
				log.Printf("[agents] skipping %s: %v", e.Name(), err)
				continue
			}
			m.agents[cfg.Name] = cfg
			log.Printf("[agents] loaded %s (model=%s)", cfg.Name, cfg.Model)
		}
	}

	log.Printf("[agents] %d agent(s) ready", len(m.agents))
	return m
}

// parseAgentFile reads a Claude Code agent .md file (YAML frontmatter + body).
func parseAgentFile(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing frontmatter")
	}
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed frontmatter")
	}

	cfg := &AgentConfig{}
	for _, line := range strings.Split(parts[0], "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "name":
			cfg.Name = strings.TrimSpace(v)
		case "description":
			cfg.Description = strings.TrimSpace(v)
		case "model":
			cfg.Model = strings.TrimSpace(v)
		case "timeout":
			secs, err := strconv.Atoi(strings.TrimSpace(v))
			if err == nil && secs > 0 {
				cfg.Timeout = time.Duration(secs) * time.Second
			}
		}
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("missing 'name' in frontmatter")
	}
	if cfg.Model == "" {
		cfg.Model = "sonnet"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 600 * time.Second
	}
	return cfg, nil
}

// HasAgent checks if an agent is registered.
func (m *Manager) HasAgent(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.agents[name]
	return ok
}

// AgentNames returns the names of all registered agents.
func (m *Manager) AgentNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.agents))
	for n := range m.agents {
		names = append(names, n)
	}
	return names
}

// SpawnAgent starts a new session for the given agent and prompt.
// Returns the response text and the Slack thread TS to use as session key.
func (m *Manager) SpawnAgent(agentName, prompt string) (string, error) {
	m.mu.RLock()
	agent, ok := m.agents[agentName]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentName)
	}
	result, err := m.runClaude(context.Background(), agent, prompt, "")
	if err != nil {
		return "", err
	}
	return result.Result, nil
}

// SpawnAgentInThread starts a new session tied to a Slack thread.
// Stores the session so follow-up messages in the thread resume it.
func (m *Manager) SpawnAgentInThread(agentName, prompt, channel, threadTS string) (string, error) {
	m.mu.RLock()
	agent, ok := m.agents[agentName]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentName)
	}

	result, err := m.runClaude(context.Background(), agent, prompt, "")
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("agent returned no result")
	}

	// Store the session keyed by Slack thread TS
	m.mu.Lock()
	m.sessions[threadTS] = &Session{
		ClaudeSessionID: result.SessionID,
		AgentID:         agentName,
		SlackThreadTS:   threadTS,
		SlackChannel:    channel,
		LastUsed:        time.Now(),
	}
	m.mu.Unlock()

	return result.Result, nil
}

// SpawnAgentInThreadStreaming starts a new session tied to a Slack thread,
// calling onUpdate with partial output as it streams in.
func (m *Manager) SpawnAgentInThreadStreaming(agentName, prompt, channel, threadTS string, onUpdate StreamCallback) (string, error) {
	m.mu.RLock()
	agent, ok := m.agents[agentName]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("agent %q not found", agentName)
	}

	result, err := m.runClaudeStreaming(context.Background(), agent, prompt, "", onUpdate)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("agent returned no result")
	}

	// Store the session keyed by Slack thread TS
	m.mu.Lock()
	m.sessions[threadTS] = &Session{
		ClaudeSessionID: result.SessionID,
		AgentID:         agentName,
		SlackThreadTS:   threadTS,
		SlackChannel:    channel,
		LastUsed:        time.Now(),
	}
	m.mu.Unlock()

	return result.Result, nil
}

// ContinueThread resumes an existing Claude session for a Slack thread reply.
func (m *Manager) ContinueThread(threadTS, message string) (string, bool, error) {
	m.mu.RLock()
	session, ok := m.sessions[threadTS]
	m.mu.RUnlock()
	if !ok {
		return "", false, nil // no session — caller should handle
	}

	m.mu.RLock()
	agent := m.agents[session.AgentID]
	m.mu.RUnlock()

	result, err := m.runClaude(context.Background(), agent, message, session.ClaudeSessionID)
	if err != nil {
		return "", true, err
	}
	if result == nil {
		return "", true, fmt.Errorf("agent returned no result")
	}

	// Update session with new Claude session ID and last used time
	m.mu.Lock()
	session.ClaudeSessionID = result.SessionID
	session.LastUsed = time.Now()
	m.mu.Unlock()

	return result.Result, true, nil
}

// runClaude executes the claude CLI and returns structured output.
// If resumeID is non-empty, resumes that session.
func (m *Manager) runClaude(ctx context.Context, agent *AgentConfig, prompt, resumeID string) (*claudeResult, error) {
	// Use model from agent definition, default to sonnet.
	model := agent.Model
	if model == "" {
		model = "sonnet"
	}

	// Allow inline model override: "opus: do something" or "sonnet: do something"
	prompt, model = parseModelOverride(prompt, model)

	args := []string{
		"--print",
		"--output-format", "json",
		"--dangerously-skip-permissions",
		"--model", model,
	}

	if resumeID != "" {
		args = append(args, "--resume", resumeID)
	}

	args = append(args, prompt)

	// Apply per-agent timeout.
	ctx, cancel := context.WithTimeout(ctx, agent.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", args...)
	// Run from $HOME so Claude loads ~/.claude agents, skills, and settings.
	cmd.Dir = os.Getenv("HOME")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("agent %s timed out after %s", agent.Name, agent.Timeout)
		}
		return nil, fmt.Errorf("agent %s failed: %w\noutput: %s", agent.Name, err, string(output))
	}

	var result claudeResult
	if err := json.Unmarshal(output, &result); err != nil {
		// Fallback: return raw output if JSON parsing fails
		return &claudeResult{Result: string(output)}, nil
	}

	if result.IsError {
		return nil, fmt.Errorf("agent error: %s", result.Result)
	}

	return &result, nil
}

// runClaudeStreaming executes the claude CLI with streaming output, calling onUpdate
// periodically (at most every 2 seconds) with accumulated output.
// At the end, it parses the final JSON result just like runClaude.
func (m *Manager) runClaudeStreaming(ctx context.Context, agent *AgentConfig, prompt, resumeID string, onUpdate StreamCallback) (*claudeResult, error) {
	model := agent.Model
	if model == "" {
		model = "sonnet"
	}

	prompt, model = parseModelOverride(prompt, model)

	args := []string{
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
		"--model", model,
	}

	if resumeID != "" {
		args = append(args, "--resume", resumeID)
	}

	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = os.Getenv("HOME")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("agent %s pipe failed: %w", agent.Name, err)
	}
	// Capture stderr separately so it doesn't pollute the stream
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agent %s start failed: %w", agent.Name, err)
	}

	// Read output line by line, accumulating text and throttling callback
	var accumulated strings.Builder
	var lastResult claudeResult
	hasResult := false

	scanner := bufio.NewScanner(stdout)
	// Allow large lines (up to 1MB)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lastUpdate := time.Time{}
	throttle := 2 * time.Second

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try to parse as a stream-json event
		var event struct {
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			Result    string `json:"result"`
			IsError   bool   `json:"is_error"`
			SessionID string `json:"session_id"`
			Content   string `json:"content"`
		}

		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not JSON — treat as raw text
			accumulated.WriteString(line)
			accumulated.WriteString("\n")
		} else {
			switch event.Type {
			case "assistant":
				// Content delta from the assistant
				if event.Content != "" {
					accumulated.WriteString(event.Content)
				}
			case "result":
				// Final result event
				lastResult = claudeResult{
					Type:      event.Type,
					Subtype:   event.Subtype,
					Result:    event.Result,
					IsError:   event.IsError,
					SessionID: event.SessionID,
				}
				hasResult = true
				continue
			case "content_block_delta":
				if event.Content != "" {
					accumulated.WriteString(event.Content)
				}
			default:
				// Other event types (system, tool_use, etc.) — skip for display
				continue
			}
		}

		// Throttled callback
		if onUpdate != nil && time.Since(lastUpdate) >= throttle {
			onUpdate(accumulated.String())
			lastUpdate = time.Now()
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("agent %s failed: %w\nstderr: %s", agent.Name, err, stderrBuf.String())
	}

	// If we got a structured result from stream-json, use it
	if hasResult {
		if lastResult.IsError {
			return nil, fmt.Errorf("agent error: %s", lastResult.Result)
		}
		return &lastResult, nil
	}

	// Fallback: try to parse the accumulated output as JSON (in case stream-json
	// isn't supported and it fell back to regular json output)
	raw := strings.TrimSpace(accumulated.String())
	var result claudeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// Return raw output
		return &claudeResult{Result: raw}, nil
	}

	if result.IsError {
		return nil, fmt.Errorf("agent error: %s", result.Result)
	}

	return &result, nil
}

// --- HTTP handlers ---

func (m *Manager) RunAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agent")

	var req struct {
		Prompt    string `json:"prompt"`
		ThreadTS  string `json:"thread_ts,omitempty"`
		Channel   string `json:"channel,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var (
		result string
		err    error
	)

	if req.ThreadTS != "" {
		result, err = m.SpawnAgentInThread(agentID, req.Prompt, req.Channel, req.ThreadTS)
	} else {
		result, err = m.SpawnAgent(agentID, req.Prompt)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"agent":  agentID,
		"result": result,
	})
}

func (m *Manager) ListAgents(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*AgentConfig, 0, len(m.agents))
	for _, a := range m.agents {
		list = append(list, a)
	}
	json.NewEncoder(w).Encode(list)
}

func (m *Manager) GetSession(w http.ResponseWriter, r *http.Request) {
	threadTS := chi.URLParam(r, "session")
	m.mu.RLock()
	session, ok := m.sessions[threadTS]
	m.mu.RUnlock()
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(session)
}

func (m *Manager) SendMessage(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agent")
	sessionID := chi.URLParam(r, "session")

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Try to continue existing session first
	result, found, err := m.ContinueThread(sessionID, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		// No session — start fresh
		result, err = m.SpawnAgent(agentID, req.Message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]string{"response": result})
}

func (m *Manager) ListSchedules(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Schedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		list = append(list, s)
	}
	json.NewEncoder(w).Encode(list)
}

func (m *Manager) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var sched Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the cron expression
	if sched.Cron == "" {
		http.Error(w, "cron expression is required", http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	m.schedCount++
	sched.ID = fmt.Sprintf("sched_%d", m.schedCount)
	sched.Status = "active"

	// Register with the cron scheduler
	agentID := sched.AgentID
	prompt := sched.Prompt
	slackCh := sched.SlackCh
	schedID := sched.ID

	entryID, err := m.cron.AddFunc(sched.Cron, func() {
		log.Printf("[scheduler] Running %s: agent=%s prompt=%q", schedID, agentID, prompt)
		result, err := m.SpawnAgent(agentID, prompt)
		if err != nil {
			log.Printf("[scheduler] %s failed: %v", schedID, err)
			result = fmt.Sprintf("Scheduled agent error: %v", err)
		}

		// Post to Slack if channel is configured
		if slackCh != "" && m.SlackPost != nil {
			msg := fmt.Sprintf("*[Scheduled] /%s*\n%s", agentID, result)
			if len(msg) > 3000 {
				msg = msg[:2950] + "\n\n_...truncated_"
			}
			if err := m.SlackPost(slackCh, msg); err != nil {
				log.Printf("[scheduler] Failed to post to Slack %s: %v", slackCh, err)
			}
		}
	})
	if err != nil {
		m.mu.Unlock()
		http.Error(w, fmt.Sprintf("invalid cron expression: %v", err), http.StatusBadRequest)
		return
	}

	sched.EntryID = entryID
	m.schedules[sched.ID] = &sched
	m.mu.Unlock()

	log.Printf("[scheduler] Created schedule %s: cron=%s agent=%s", sched.ID, sched.Cron, sched.AgentID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sched)
}

// parseModelOverride checks if the prompt starts with a model name prefix.
// e.g. "opus: write me a campaign" → model="opus", prompt="write me a campaign"
func parseModelOverride(prompt, defaultModel string) (string, string) {
	models := map[string]string{
		"opus:":   "opus",
		"sonnet:": "sonnet",
		"haiku:":  "haiku",
	}
	for prefix, model := range models {
		if len(prompt) > len(prefix) && strings.EqualFold(prompt[:len(prefix)], prefix) {
			return strings.TrimSpace(prompt[len(prefix):]), model
		}
	}
	return prompt, defaultModel
}

func (m *Manager) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m.mu.Lock()
	defer m.mu.Unlock()
	sched, ok := m.schedules[id]
	if !ok {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}
	m.cron.Remove(sched.EntryID)
	delete(m.schedules, id)
	log.Printf("[scheduler] Deleted schedule %s", id)
	w.WriteHeader(http.StatusNoContent)
}
