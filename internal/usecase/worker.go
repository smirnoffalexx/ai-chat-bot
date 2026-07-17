package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/i18n"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

// WorkerPool consumes tasks from the queue and processes them with the agent.
// Fast tasks are answered directly; if a task is still running after ackAfter,
// the user gets a "working on it" note and the answer follows when ready.
type WorkerPool struct {
	queue        port.Queue
	repo         port.TaskRepository
	history      port.History
	lang         port.LanguageStore
	sender       port.Sender
	agent        *Agent
	systemPrompt string
	historyLimit int
	ackAfter     time.Duration
	log          *slog.Logger
}

func NewWorkerPool(
	queue port.Queue,
	repo port.TaskRepository,
	history port.History,
	lang port.LanguageStore,
	sender port.Sender,
	agent *Agent,
	systemPrompt string,
	historyLimit int,
	ackAfter time.Duration,
	log *slog.Logger,
) *WorkerPool {
	return &WorkerPool{
		queue:        queue,
		repo:         repo,
		history:      history,
		lang:         lang,
		sender:       sender,
		agent:        agent,
		systemPrompt: systemPrompt,
		historyLimit: historyLimit,
		ackAfter:     ackAfter,
		log:          log,
	}
}

// Start launches n workers and blocks until ctx is cancelled.
func (w *WorkerPool) Start(ctx context.Context, n int) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.loop(ctx, id)
		}(i)
	}
	wg.Wait()
}

func (w *WorkerPool) loop(ctx context.Context, id int) {
	for {
		task, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			w.log.Error("dequeue", "worker", id, "err", err)
			continue
		}
		w.process(ctx, task)
	}
}

func (w *WorkerPool) process(ctx context.Context, task domain.Task) {
	task.Status = domain.TaskRunning
	task.StartedAt = time.Now()
	_ = w.repo.Save(ctx, task)

	lang := w.lang.Get(task.ChatID)

	// Delayed acknowledgement: only nag the user if the work is actually slow.
	ackCtx, cancelAck := context.WithCancel(ctx)
	defer cancelAck()
	go w.ackIfSlow(ackCtx, task.ChatID, lang)

	// Build the conversation: system prompt + language directive + bounded
	// history + this prompt.
	convo := []domain.Message{
		{Role: domain.RoleSystem, Content: w.systemPrompt},
		{Role: domain.RoleSystem, Content: i18n.ReplyDirective(lang)},
	}
	convo = append(convo, w.history.Get(task.ChatID)...)
	userMsg := domain.Message{Role: domain.RoleUser, Content: task.Prompt}
	convo = append(convo, userMsg)

	answer, _, err := w.agent.Run(ctx, convo)
	cancelAck()

	task.EndedAt = time.Now()
	if err != nil {
		task.Status = domain.TaskFailed
		task.Err = err.Error()
		_ = w.repo.Save(ctx, task)
		w.log.Error("task failed", "id", task.ID, "err", err)
		_ = w.sender.SendText(ctx, task.ChatID, fmt.Sprintf(i18n.T(lang, i18n.ErrorPrefix), err.Error()))
		return
	}

	task.Status = domain.TaskDone
	task.Result = answer
	_ = w.repo.Save(ctx, task)

	if answer == "" {
		answer = i18n.T(lang, i18n.NothingToReport)
	}

	// Persist only the user turn and the final assistant answer to memory.
	// Intermediate tool calls/results are intentionally not stored: keeping them
	// risks orphaned tool messages after trimming, which the LLM API rejects.
	w.history.Append(task.ChatID, userMsg)
	w.history.Append(task.ChatID, domain.Message{Role: domain.RoleAssistant, Content: answer})
	w.trimHistory(task.ChatID)

	if err := w.sender.SendText(ctx, task.ChatID, answer); err != nil {
		w.log.Error("send answer", "id", task.ID, "err", err)
	}
}

func (w *WorkerPool) ackIfSlow(ctx context.Context, chatID domain.ChatID, lang domain.Language) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(w.ackAfter):
		_ = w.sender.SendText(ctx, chatID, i18n.T(lang, i18n.Working))
	}
}

// trimHistory keeps only the most recent historyLimit messages for a chat.
func (w *WorkerPool) trimHistory(chatID domain.ChatID) {
	msgs := w.history.Get(chatID)
	if len(msgs) <= w.historyLimit {
		return
	}
	w.history.Reset(chatID)
	w.history.Append(chatID, msgs[len(msgs)-w.historyLimit:]...)
}
