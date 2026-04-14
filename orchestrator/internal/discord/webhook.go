package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EmbedField is a single field in a Discord embed.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Embed is a Discord rich-embed object.
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
}

// EmbedFooter is the footer of a Discord embed.
type EmbedFooter struct {
	Text string `json:"text"`
}

// WebhookPayload is the body sent to a Discord webhook.
type WebhookPayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

// Client posts messages to a Discord webhook URL.
type Client struct {
	URL        string
	httpClient *http.Client
}

// NewClient creates a Discord webhook client.
func NewClient(webhookURL string) *Client {
	return &Client{
		URL:        webhookURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Send posts a WebhookPayload to Discord. Discord allows at most 10 embeds
// per message; callers should split larger payloads.
func (c *Client) Send(ctx context.Context, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned %d", resp.StatusCode)
	}
	return nil
}
