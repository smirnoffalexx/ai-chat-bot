// Command bot runs the Telegram AI-agent bot.
//
// Architecture (clean / hexagonal):
//
//	domain   – pure entities (no dependencies)
//	port     – interfaces the core depends on
//	usecase  – application logic: agent loop, handler, worker pool
//	adapter  – implementations: telegram, anthropic, in-memory storage
//	auth     – user whitelist
//
// Dependencies point inward only; main is the composition root that wires the
// concrete adapters into the use cases.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smirnoffalexx/ai-chat-bot/internal/adapter/anthropic"
	"github.com/smirnoffalexx/ai-chat-bot/internal/adapter/memory"
	"github.com/smirnoffalexx/ai-chat-bot/internal/adapter/telegram"
	"github.com/smirnoffalexx/ai-chat-bot/internal/agent/tools"
	"github.com/smirnoffalexx/ai-chat-bot/internal/auth"
	"github.com/smirnoffalexx/ai-chat-bot/internal/config"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
	"github.com/smirnoffalexx/ai-chat-bot/internal/usecase"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := config.Load()

	// --- adapters ---
	llm := anthropic.New(anthropic.Config{
		APIKey:    cfg.Anthropic.APIKey,
		BaseURL:   cfg.Anthropic.BaseURL,
		Model:     cfg.Anthropic.Model,
		MaxTokens: cfg.Anthropic.MaxTokens,
	})
	tg := telegram.New(cfg.Telegram.Token, cfg.Telegram.PollTimeoutSeconds, log)
	queue := memory.NewQueue(256)
	repo := memory.NewTaskRepository()
	history := memory.NewHistory()
	languages := memory.NewLanguageStore()
	authz := auth.NewWhitelist(cfg.AllowedUserIDs)

	// --- agent tools ---
	agentTools := []port.Tool{
		tools.CurrentTime{},
		tools.NewHTTPGet(),
	}

	// --- use cases ---
	agent := usecase.NewAgent(llm, agentTools, cfg.Agent.MaxSteps, log)
	workers := usecase.NewWorkerPool(
		queue, repo, history, languages, tg, agent,
		cfg.Agent.SystemPrompt,
		cfg.Agent.HistoryLimit,
		time.Duration(cfg.Agent.AckAfterSeconds)*time.Second,
		log,
	)
	handler := usecase.NewHandler(authz, queue, repo, history, languages, tg, log)

	// --- run ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go workers.Start(ctx, cfg.Workers)

	log.Info("bot started",
		"model", cfg.Anthropic.Model,
		"workers", cfg.Workers,
		"allowed_users", len(cfg.AllowedUserIDs),
	)

	tg.Poll(ctx, telegram.Handlers{
		Message:  handler.Handle,
		Callback: handler.HandleCallback,
	})

	log.Info("shutting down")
}
