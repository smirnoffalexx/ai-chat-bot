// Package openai implements the port.LLM interface against the OpenAI
// Chat Completions API using only the standard library.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

// Client calls the OpenAI Chat Completions API.
type Client struct {
	http        *http.Client
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	maxTokens   int
}

// Config configures a Client.
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
}

func New(cfg Config) *Client {
	return &Client{
		http:        &http.Client{Timeout: 5 * time.Minute},
		apiKey:      cfg.APIKey,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
	}
}

var _ port.LLM = (*Client)(nil)

// Chat sends the conversation and returns the model's reply message.
func (c *Client) Chat(ctx context.Context, req port.ChatRequest) (domain.Message, error) {
	body := chatRequest{
		Model:       c.model,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		Messages:    toWireMessages(req.Messages),
		Tools:       toWireTools(req.Tools),
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return domain.Message{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return domain.Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return domain.Message{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return domain.Message{}, fmt.Errorf("openai status %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return domain.Message{}, fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return domain.Message{}, fmt.Errorf("openai returned no choices")
	}
	return fromWireMessage(parsed.Choices[0].Message), nil
}

// --- wire types ---

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Tools       []wireTool    `json:"tools,omitempty"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message wireMessage `json:"message"`
	} `json:"choices"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFuncSpec `json:"function"`
}

type wireFuncSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// --- conversions ---

func toWireMessages(msgs []domain.Message) []wireMessage {
	out := make([]wireMessage, 0, len(msgs))
	for _, m := range msgs {
		wm := wireMessage{Role: string(m.Role)}
		switch m.Role {
		case domain.RoleAssistant:
			// Assistant messages with tool calls may have empty content, which
			// must be sent as JSON null rather than "".
			if m.Content != "" {
				wm.Content = strptr(m.Content)
			}
			for _, tc := range m.ToolCalls {
				w := wireToolCall{ID: tc.ID, Type: "function"}
				w.Function.Name = tc.Name
				w.Function.Arguments = argsToString(tc.Arguments)
				wm.ToolCalls = append(wm.ToolCalls, w)
			}
		case domain.RoleTool:
			wm.Content = strptr(m.Content)
			wm.ToolCallID = m.ToolCallID
			wm.Name = m.Name
		default:
			wm.Content = strptr(m.Content)
		}
		out = append(out, wm)
	}
	return out
}

func fromWireMessage(m wireMessage) domain.Message {
	dm := domain.Message{Role: domain.Role(m.Role)}
	if m.Content != nil {
		dm.Content = *m.Content
	}
	for _, tc := range m.ToolCalls {
		args := tc.Function.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		dm.ToolCalls = append(dm.ToolCalls, domain.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(args),
		})
	}
	return dm
}

func toWireTools(specs []domain.ToolSpec) []wireTool {
	if len(specs) == 0 {
		return nil
	}
	out := make([]wireTool, 0, len(specs))
	for _, s := range specs {
		params := s.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, wireTool{
			Type: "function",
			Function: wireFuncSpec{
				Name:        s.Name,
				Description: s.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

func argsToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

func strptr(s string) *string { return &s }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
