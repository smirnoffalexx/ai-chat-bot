package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

func TestChatWireFormat(t *testing.T) {
	var captured map[string]any
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"content":[{"type":"text","text":"Done. "},{"type":"tool_use","id":"tu_1","name":"get_time","input":{"tz":"UTC"}}]}`)
	}))
	defer srv.Close()

	c := New(Config{APIKey: "sk-test", BaseURL: srv.URL, Model: "claude-opus-4-8", MaxTokens: 512})

	// A conversation exercising: two system messages, history, an assistant
	// turn with a tool call, and two parallel tool results that must merge.
	convo := []domain.Message{
		{Role: domain.RoleSystem, Content: "You are helpful."},
		{Role: domain.RoleSystem, Content: "Reply in English."},
		{Role: domain.RoleUser, Content: "hi"},
		{Role: domain.RoleAssistant, Content: "", ToolCalls: []domain.ToolCall{
			{ID: "a", Name: "t1", Arguments: json.RawMessage(`{"x":1}`)},
			{ID: "b", Name: "t2", Arguments: nil},
		}},
		{Role: domain.RoleTool, ToolCallID: "a", Name: "t1", Content: "res-a"},
		{Role: domain.RoleTool, ToolCallID: "b", Name: "t2", Content: "res-b"},
	}
	specs := []domain.ToolSpec{{Name: "get_time", Description: "time", Parameters: nil}}

	reply, err := c.Chat(context.Background(), port.ChatRequest{Messages: convo, Tools: specs})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Headers
	if gotHeaders.Get("X-Api-Key") != "sk-test" || gotHeaders.Get("Anthropic-Version") != apiVersion {
		t.Fatalf("bad headers: %v", gotHeaders)
	}

	// System extracted and joined
	if captured["system"] != "You are helpful.\n\nReply in English." {
		t.Fatalf("system = %q", captured["system"])
	}

	// Messages: user, assistant(tool_use x2), user(tool_result x2 merged)
	msgs := captured["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("want 3 wire messages, got %d: %#v", len(msgs), msgs)
	}
	asst := msgs[1].(map[string]any)
	if asst["role"] != "assistant" || len(asst["content"].([]any)) != 2 {
		t.Fatalf("assistant turn wrong: %#v", asst)
	}
	// nil args must become {}
	tuB := asst["content"].([]any)[1].(map[string]any)
	if in, _ := json.Marshal(tuB["input"]); string(in) != "{}" {
		t.Fatalf("empty tool_use input = %s", in)
	}
	toolTurn := msgs[2].(map[string]any)
	if toolTurn["role"] != "user" || len(toolTurn["content"].([]any)) != 2 {
		t.Fatalf("tool results not merged into one user turn: %#v", toolTurn)
	}
	trBlock := toolTurn["content"].([]any)[0].(map[string]any)
	if trBlock["type"] != "tool_result" || trBlock["tool_use_id"] != "a" || trBlock["content"] != "res-a" {
		t.Fatalf("tool_result block wrong: %#v", trBlock)
	}

	// Tool spec sent as input_schema (defaulted from nil)
	tools := captured["tools"].([]any)
	sch := tools[0].(map[string]any)["input_schema"].(map[string]any)
	if sch["type"] != "object" {
		t.Fatalf("input_schema not defaulted: %#v", sch)
	}

	// Response parsed: text concatenated, tool call extracted
	if reply.Role != domain.RoleAssistant || reply.Content != "Done. " {
		t.Fatalf("reply content = %q", reply.Content)
	}
	if len(reply.ToolCalls) != 1 || reply.ToolCalls[0].Name != "get_time" ||
		string(reply.ToolCalls[0].Arguments) != `{"tz":"UTC"}` {
		t.Fatalf("reply tool calls wrong: %#v", reply.ToolCalls)
	}
}
