package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

const (
	elevenLabsBaseURL = "https://api.elevenlabs.io/v1"
)

// Client wraps the ElevenLabs API.
type Client struct {
	APIKey  string
	VoiceID string
	http    *http.Client
}

// NewClient creates a new ElevenLabs client.
func NewClient() *Client {
	return &Client{
		APIKey:  os.Getenv("ELEVENLABS_API_KEY"),
		VoiceID: os.Getenv("ELEVENLABS_VOICE_ID"),
		http:    &http.Client{},
	}
}

// IsConfigured returns true if API key is set.
func (c *Client) IsConfigured() bool {
	return c.APIKey != ""
}

// TextToSpeech converts text to audio bytes (mp3).
func (c *Client) TextToSpeech(text string) ([]byte, error) {
	voiceID := c.VoiceID
	if voiceID == "" {
		voiceID = "onwK4e9ZLuTAKqWW03F9" // "Daniel" - Steady Broadcaster (premade/free)
	}

	url := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsBaseURL, voiceID)

	body, _ := json.Marshal(map[string]interface{}{
		"text":     text,
		"model_id": "eleven_multilingual_v2",
		"voice_settings": map[string]float64{
			"stability":        0.5,
			"similarity_boost": 0.75,
		},
	})

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elevenlabs TTS error %d: %s", resp.StatusCode, string(errBody))
	}

	return io.ReadAll(resp.Body)
}

// SpeechToText converts audio to text using ElevenLabs STT.
// Accepts audio bytes and the original content type from the upload.
func (c *Client) SpeechToText(audio []byte, filename string) (string, error) {
	url := fmt.Sprintf("%s/speech-to-text", elevenLabsBaseURL)

	// ElevenLabs STT expects multipart form upload
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}

	// model_id field
	if err := writer.WriteField("model_id", "scribe_v1"); err != nil {
		return "", err
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("xi-api-key", c.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("elevenlabs STT error %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Text, nil
}
