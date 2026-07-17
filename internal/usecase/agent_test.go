package usecase

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

// fakeLLM returns queued replies in order, recording what it received.
type fakeLLM struct {
	replies []domain.Message
	calls   int
	last    port.ChatRequest
}

func (f *fakeLLM) Chat(_ context.Context, req port.ChatRequest) (domain.Message, error) {
	f.last = req
	r := f.replies[f.calls]
	f.calls++
	return r, nil
}

// echoTool returns the "value" argument it was given.
type echoTool struct{ executed bool }

func (e *echoTool) Name() string            { return "echo" }
func (e *echoTool) Description() string     { return "echoes input" }
func (e *echoTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (e *echoTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	e.executed = true
	var in struct {
		Value string `json:"value"`
	}
	_ = json.Unmarshal(args, &in)
	return "echoed:" + in.Value, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAgentRunsToolThenAnswers(t *testing.T) {
	tool := &echoTool{}
	llm := &fakeLLM{replies: []domain.Message{
		// Turn 1: request a tool call.
		{Role: domain.RoleAssistant, ToolCalls: []domain.ToolCall{
			{ID: "c1", Name: "echo", Arguments: json.RawMessage(`{"value":"hi"}`)},
		}},
		// Turn 2: final answer after seeing the tool result.
		{Role: domain.RoleAssistant, Content: "done"},
	}}

	agent := NewAgent(llm, []port.Tool{tool}, 5, testLogger())
	answer, produced, err := agent.Run(context.Background(), []domain.Message{
		{Role: domain.RoleUser, Content: "please echo hi"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "done" {
		t.Fatalf("answer = %q, want %q", answer, "done")
	}
	if !tool.executed {
		t.Fatal("tool was not executed")
	}
	if llm.calls != 2 {
		t.Fatalf("llm called %d times, want 2", llm.calls)
	}
	// produced should contain: assistant(toolcall), tool(result), assistant(done)
	if len(produced) != 3 {
		t.Fatalf("produced %d messages, want 3", len(produced))
	}
	if produced[1].Role != domain.RoleTool || produced[1].Content != "echoed:hi" {
		t.Fatalf("tool result message wrong: %+v", produced[1])
	}
}

func TestAgentExceedsMaxSteps(t *testing.T) {
	// Always ask for a tool call, never finish.
	loop := domain.Message{Role: domain.RoleAssistant, ToolCalls: []domain.ToolCall{
		{ID: "c", Name: "echo", Arguments: json.RawMessage(`{}`)},
	}}
	llm := &fakeLLM{replies: []domain.Message{loop, loop, loop}}

	agent := NewAgent(llm, []port.Tool{&echoTool{}}, 2, testLogger())
	if _, _, err := agent.Run(context.Background(), []domain.Message{
		{Role: domain.RoleUser, Content: "go"},
	}); err == nil {
		t.Fatal("expected max-steps error, got nil")
	}
}

func TestAgentUnknownToolIsReportedNotFatal(t *testing.T) {
	llm := &fakeLLM{replies: []domain.Message{
		{Role: domain.RoleAssistant, ToolCalls: []domain.ToolCall{
			{ID: "c1", Name: "missing", Arguments: json.RawMessage(`{}`)},
		}},
		{Role: domain.RoleAssistant, Content: "recovered"},
	}}
	agent := NewAgent(llm, nil, 5, testLogger())
	answer, produced, err := agent.Run(context.Background(), []domain.Message{
		{Role: domain.RoleUser, Content: "go"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "recovered" {
		t.Fatalf("answer = %q, want recovered", answer)
	}
	// The tool-result message should carry the unknown-tool error back to the model.
	if got := produced[1].Content; got == "" || got[:5] != "error" {
		t.Fatalf("expected error result, got %q", got)
	}
}
