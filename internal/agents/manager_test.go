package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marketing.md")
	content := `---
name: marketing
description: Full-stack marketing agent
model: sonnet
---

You are a marketing expert.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "marketing" {
		t.Errorf("expected name 'marketing', got %q", cfg.Name)
	}
	if cfg.Description != "Full-stack marketing agent" {
		t.Errorf("expected description, got %q", cfg.Description)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("expected model 'sonnet', got %q", cfg.Model)
	}
}

func TestParseAgentFile_DefaultModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := `---
name: test-agent
description: A test agent
---

System prompt here.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %q", cfg.Model)
	}
}

func TestParseAgentFile_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	content := `---
description: No name field
---

Body.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := parseAgentFile(path)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestParseAgentFile_MissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nofm.md")
	content := `Just a plain markdown file without frontmatter.`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := parseAgentFile(path)
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseAgentFile_MalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.md")
	content := `---
name: test
no closing frontmatter delimiter
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := parseAgentFile(path)
	if err == nil {
		t.Error("expected error for malformed frontmatter")
	}
}

func TestParseModelOverride(t *testing.T) {
	tests := []struct {
		prompt       string
		defaultModel string
		wantPrompt   string
		wantModel    string
	}{
		{"opus: write a campaign", "sonnet", "write a campaign", "opus"},
		{"sonnet: analyze this", "opus", "analyze this", "sonnet"},
		{"haiku: quick answer", "sonnet", "quick answer", "haiku"},
		{"OPUS: uppercase test", "sonnet", "uppercase test", "opus"},
		{"just a normal prompt", "sonnet", "just a normal prompt", "sonnet"},
		{"opus:", "sonnet", "opus:", "sonnet"}, // No text after prefix
		{"", "sonnet", "", "sonnet"},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			gotPrompt, gotModel := parseModelOverride(tt.prompt, tt.defaultModel)
			if gotPrompt != tt.wantPrompt {
				t.Errorf("prompt: got %q, want %q", gotPrompt, tt.wantPrompt)
			}
			if gotModel != tt.wantModel {
				t.Errorf("model: got %q, want %q", gotModel, tt.wantModel)
			}
		})
	}
}

func TestManager_HasAgent(t *testing.T) {
	m := &Manager{
		agents: map[string]*AgentConfig{
			"marketing": {Name: "marketing", Model: "sonnet"},
			"pentester": {Name: "pentester", Model: "opus"},
		},
	}

	if !m.HasAgent("marketing") {
		t.Error("expected HasAgent to return true for 'marketing'")
	}
	if m.HasAgent("nonexistent") {
		t.Error("expected HasAgent to return false for 'nonexistent'")
	}
}

func TestManager_AgentNames(t *testing.T) {
	m := &Manager{
		agents: map[string]*AgentConfig{
			"marketing": {Name: "marketing"},
			"pentester": {Name: "pentester"},
			"youtube":   {Name: "youtube"},
		},
	}

	names := m.AgentNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"marketing", "pentester", "youtube"} {
		if !nameSet[expected] {
			t.Errorf("expected %q in agent names", expected)
		}
	}
}

func TestManager_SpawnAgent_NotFound(t *testing.T) {
	m := &Manager{
		agents: map[string]*AgentConfig{},
	}

	_, err := m.SpawnAgent("nonexistent", "hello")
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestManager_ContinueThread_NoSession(t *testing.T) {
	m := &Manager{
		sessions: make(map[string]*Session),
	}

	_, found, err := m.ContinueThread("no-such-thread", "hello")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false for nonexistent thread")
	}
}
