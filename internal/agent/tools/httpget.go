package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPGet fetches a URL and returns its (truncated) text body. It is a simple
// example of a tool that lets the agent gather information to solve a task.
type HTTPGet struct {
	client  *http.Client
	maxSize int64
}

func NewHTTPGet() *HTTPGet {
	return &HTTPGet{
		client:  &http.Client{Timeout: 20 * time.Second},
		maxSize: 200 << 10, // 200 KiB
	}
}

func (h *HTTPGet) Name() string { return "http_get" }

func (h *HTTPGet) Description() string {
	return "Fetches the given HTTP(S) URL with a GET request and returns the response " +
		"body as text (truncated). Useful for reading web pages or JSON APIs."
}

func (h *HTTPGet) Schema() json.RawMessage {
	return mustJSON(schema{
		"type": "object",
		"properties": schema{
			"url": schema{
				"type":        "string",
				"description": "The absolute http:// or https:// URL to fetch.",
			},
		},
		"required": []string{"url"},
	})
}

func (h *HTTPGet) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return "", fmt.Errorf("url must start with http:// or https://")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ai-chat-bot/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, h.maxSize))
	return fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(body)), nil
}
