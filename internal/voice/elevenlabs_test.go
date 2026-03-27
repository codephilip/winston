package voice

import (
	"os"
	"testing"
)

func TestNewClient_Configured(t *testing.T) {
	os.Setenv("ELEVENLABS_API_KEY", "test-key")
	os.Setenv("ELEVENLABS_VOICE_ID", "test-voice")
	defer os.Unsetenv("ELEVENLABS_API_KEY")
	defer os.Unsetenv("ELEVENLABS_VOICE_ID")

	c := NewClient()
	if !c.IsConfigured() {
		t.Error("expected client to be configured")
	}
	if c.APIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %q", c.APIKey)
	}
	if c.VoiceID != "test-voice" {
		t.Errorf("expected voice ID 'test-voice', got %q", c.VoiceID)
	}
}

func TestNewClient_NotConfigured(t *testing.T) {
	os.Unsetenv("ELEVENLABS_API_KEY")
	os.Unsetenv("ELEVENLABS_VOICE_ID")

	c := NewClient()
	if c.IsConfigured() {
		t.Error("expected client to not be configured")
	}
}

func TestClient_IsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   bool
	}{
		{"with key", "some-key", true},
		{"empty key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{APIKey: tt.apiKey}
			if got := c.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}
