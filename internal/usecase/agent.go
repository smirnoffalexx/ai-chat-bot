// Package usecase holds the application core: agent reasoning, request
// handling, and background task processing. It depends only on domain and port.
package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

// Agent runs a tool-using reasoning loop against an LLM to solve a task.
type Agent struct {
	llm      port.LLM
	tools    map[string]port.Tool
	specs    []domain.ToolSpec
	maxSteps int
	log      *slog.Logger
}

// NewAgent builds an agent from an LLM and a set of tools.
func NewAgent(llm port.LLM, tools []port.Tool, maxSteps int, log *slog.Logger) *Agent {
	m := make(map[string]port.Tool, len(tools))
	specs := make([]domain.ToolSpec, 0, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
		specs = append(specs, domain.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Schema(),
		})
	}
	return &Agent{llm: llm, tools: m, specs: specs, maxSteps: maxSteps, log: log}
}

// Run drives the conversation to a final assistant answer, executing any tool
// calls the model requests along the way. It returns the final text plus the
// new messages produced (assistant turns and tool results) so the caller can
// persist them to conversation history.
func (a *Agent) Run(ctx context.Context, conversation []domain.Message) (answer string, produced []domain.Message, err error) {
	msgs := append([]domain.Message(nil), conversation...)

	for step := 0; step < a.maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return "", produced, err
		}

		reply, err := a.llm.Chat(ctx, port.ChatRequest{Messages: msgs, Tools: a.specs})
		if err != nil {
			return "", produced, fmt.Errorf("llm chat (step %d): %w", step, err)
		}
		msgs = append(msgs, reply)
		produced = append(produced, reply)

		if len(reply.ToolCalls) == 0 {
			return reply.Content, produced, nil
		}

		for _, call := range reply.ToolCalls {
			result := a.execute(ctx, call)
			msgs = append(msgs, result)
			produced = append(produced, result)
		}
	}

	return "", produced, fmt.Errorf("agent exceeded max steps (%d) without a final answer", a.maxSteps)
}

// execute runs a single tool call, converting any error into a tool-result
// message so the model can observe and recover from the failure.
func (a *Agent) execute(ctx context.Context, call domain.ToolCall) domain.Message {
	toolMsg := domain.Message{
		Role:       domain.RoleTool,
		ToolCallID: call.ID,
		Name:       call.Name,
	}

	tool, ok := a.tools[call.Name]
	if !ok {
		toolMsg.Content = fmt.Sprintf("error: unknown tool %q", call.Name)
		return toolMsg
	}

	a.log.Info("tool call", "tool", call.Name, "args", string(call.Arguments))
	out, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		a.log.Warn("tool failed", "tool", call.Name, "err", err)
		toolMsg.Content = "error: " + err.Error()
		return toolMsg
	}
	toolMsg.Content = out
	return toolMsg
}
