package domain

import "encoding/json"

// Role identifies the author of a chat message in an LLM conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in an LLM conversation. It is deliberately
// provider-agnostic; adapters translate it to/from a vendor wire format.
type Message struct {
	Role    Role
	Content string

	// ToolCalls is set on assistant messages that request tool execution.
	ToolCalls []ToolCall

	// ToolCallID and Name are set on RoleTool messages carrying a tool result.
	ToolCallID string
	Name       string
}

// ToolCall is the model's request to invoke a tool with JSON arguments.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// ToolSpec advertises a tool to the model: a name, a description, and a
// JSON-Schema object describing its parameters.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}
