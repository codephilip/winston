package slack

import (
	"testing"
)

func TestParseAgentFromText(t *testing.T) {
	agentNames := []string{"marketing", "pentester", "youtube"}

	tests := []struct {
		name      string
		text      string
		wantAgent string
		wantText  string
	}{
		{"slash prefix", "/marketing run a campaign", "marketing", "run a campaign"},
		{"colon prefix", "pentester: scan the network", "pentester", "scan the network"},
		{"space prefix", "youtube create a video script", "youtube", "create a video script"},
		{"no match", "hello world", "", "hello world"},
		{"empty", "", "", ""},
		{"case insensitive", "/Marketing run it", "marketing", "run it"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, text := parseAgentFromText(tt.text, agentNames)
			if agent != tt.wantAgent {
				t.Errorf("agent: got %q, want %q", agent, tt.wantAgent)
			}
			if text != tt.wantText {
				t.Errorf("text: got %q, want %q", text, tt.wantText)
			}
		})
	}
}
