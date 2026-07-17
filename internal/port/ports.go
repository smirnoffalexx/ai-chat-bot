// Package port declares the interfaces (ports) that the application core
// depends on. Adapters in internal/adapter implement these; the use cases in
// internal/usecase consume them. Dependencies point inward only.
package port

import (
	"context"
	"encoding/json"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// LLM is a chat-completion model that may request tool calls.
type LLM interface {
	Chat(ctx context.Context, req ChatRequest) (domain.Message, error)
}

// ChatRequest is one completion request: the running conversation plus the
// tools the model is allowed to call this turn.
type ChatRequest struct {
	Messages []domain.Message
	Tools    []domain.ToolSpec
}

// Sender delivers messages back to the user on the messaging platform.
type Sender interface {
	SendText(ctx context.Context, chatID domain.ChatID, text string) error
	SendTyping(ctx context.Context, chatID domain.ChatID) error
	// SendMenu sends text with a row of tappable inline buttons.
	SendMenu(ctx context.Context, chatID domain.ChatID, text string, buttons []domain.MenuButton) error
	// AnswerCallback acknowledges a button press (dismisses the loading spinner).
	AnswerCallback(ctx context.Context, callbackID, text string) error
}

// LanguageStore persists each chat's chosen language.
type LanguageStore interface {
	// Get returns the chat's language, or domain.DefaultLanguage if unset.
	Get(chatID domain.ChatID) domain.Language
	Set(chatID domain.ChatID, lang domain.Language)
}

// Queue is a producer/consumer job queue for agent tasks.
type Queue interface {
	Enqueue(ctx context.Context, t domain.Task) error
	// Dequeue blocks until a task is available or ctx is cancelled.
	Dequeue(ctx context.Context) (domain.Task, error)
}

// TaskRepository persists task state for status lookups and auditing.
type TaskRepository interface {
	Save(ctx context.Context, t domain.Task) error
	Get(ctx context.Context, id string) (domain.Task, bool)
}

// History stores bounded per-chat conversation memory so the bot is multi-turn.
type History interface {
	Append(chatID domain.ChatID, msgs ...domain.Message)
	Get(chatID domain.ChatID) []domain.Message
	Reset(chatID domain.ChatID)
}

// Tool is a capability the agent can invoke to solve a task.
type Tool interface {
	Name() string
	Description() string
	// Schema returns a JSON-Schema object describing the tool's arguments.
	Schema() json.RawMessage
	// Execute runs the tool and returns a result string for the model.
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Authorizer decides whether a user is allowed to use the bot.
type Authorizer interface {
	Allowed(id domain.UserID) bool
}
