// Package anthropic implements the port.LLM interface against the Anthropic
// (Claude) Messages API using only the standard library.
package anthropic

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

// apiVersion is the required anthropic-version header value.
const apiVersion = "2023-06-01"

// Client calls the Anthropic Messages API.
type Client struct {
	http      *http.Client
	apiKey    string
	baseURL   string
	model     string
	maxTokens int
}

// Config configures a Client.
type Config struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int
}

func New(cfg Config) *Client {
	return &Client{
		http:      &http.Client{Timeout: 5 * time.Minute},
		apiKey:    cfg.APIKey,
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
	}
}

var _ port.LLM = (*Client)(nil)

// Chat sends the conversation and returns the model's reply message.
//
// The Anthropic Messages API differs from OpenAI's Chat Completions in ways the
// conversion below bridges: the system prompt is a top-level field rather than a
// message, and tool calls/results are content blocks rather than dedicated
// fields. Note there is no temperature parameter — current Claude models reject
// it; behaviour is steered via the system prompt instead.
func (c *Client) Chat(ctx context.Context, req port.ChatRequest) (domain.Message, error) {
	system, messages := toWireMessages(req.Messages)
	body := chatRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    system,
		Messages:  messages,
		Tools:     toWireTools(req.Tools),
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return domain.Message{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return domain.Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.apiKey)
	httpReq.Header.Set("Anthropic-Version", apiVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return domain.Message{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return domain.Message{}, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return domain.Message{}, fmt.Errorf("decode response: %w", err)
	}
	return fromWireResponse(parsed), nil
}

// --- wire types ---

type chatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system,omitempty"`
	Messages  []wireMessage `json:"messages"`
	Tools     []wireTool    `json:"tools,omitempty"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`

	// toolResult marks a synthesized user turn carrying tool_result blocks, so
	// consecutive tool results can be merged into a single user message (the API
	// requires all results for one assistant turn in one user message).
	toolResult bool
}

type textBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

type toolUseBlock struct {
	Type  string          `json:"type"` // "tool_use"
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
	Type      string `json:"type"` // "tool_result"
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

type wireTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type chatResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
}

// --- conversions ---

// toWireMessages splits the provider-agnostic conversation into Anthropic's
// top-level system string and its alternating user/assistant message list.
func toWireMessages(msgs []domain.Message) (system string, out []wireMessage) {
	var sys []string
	for _, m := range msgs {
		switch m.Role {
		case domain.RoleSystem:
			if m.Content != "" {
				sys = append(sys, m.Content)
			}
		case domain.RoleUser:
			out = append(out, wireMessage{
				Role:    "user",
				Content: []any{textBlock{Type: "text", Text: m.Content}},
			})
		case domain.RoleAssistant:
			var blocks []any
			if m.Content != "" {
				blocks = append(blocks, textBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := tc.Arguments
				if len(input) == 0 {
					input = json.RawMessage(`{}`)
				}
				blocks = append(blocks, toolUseBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			out = append(out, wireMessage{Role: "assistant", Content: blocks})
		case domain.RoleTool:
			block := toolResultBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}
			// Merge into the preceding synthesized tool-result user turn if there
			// is one; otherwise start a new user turn.
			if n := len(out); n > 0 && out[n-1].toolResult {
				out[n-1].Content = append(out[n-1].Content, block)
			} else {
				out = append(out, wireMessage{
					Role:       "user",
					Content:    []any{block},
					toolResult: true,
				})
			}
		}
	}
	return strings.Join(sys, "\n\n"), out
}

// fromWireResponse flattens the response content blocks into a single assistant
// message: text blocks become the reply text, tool_use blocks become tool calls.
func fromWireResponse(resp chatResponse) domain.Message {
	dm := domain.Message{Role: domain.RoleAssistant}
	var text strings.Builder
	for _, b := range resp.Content {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "tool_use":
			args := b.Input
			if len(args) == 0 {
				args = json.RawMessage("{}")
			}
			dm.ToolCalls = append(dm.ToolCalls, domain.ToolCall{
				ID:        b.ID,
				Name:      b.Name,
				Arguments: json.RawMessage(args),
			})
		}
	}
	dm.Content = text.String()
	return dm
}

func toWireTools(specs []domain.ToolSpec) []wireTool {
	if len(specs) == 0 {
		return nil
	}
	out := make([]wireTool, 0, len(specs))
	for _, s := range specs {
		schema := s.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, wireTool{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: schema,
		})
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
