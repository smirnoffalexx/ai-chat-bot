package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// Config holds all runtime configuration, loaded from the environment.
type Config struct {
	Telegram  TelegramConfig
	Anthropic AnthropicConfig
	Agent     AgentConfig

	// AllowedUserIDs is the whitelist of Telegram user IDs permitted to chat.
	AllowedUserIDs []domain.UserID
	// Workers is the number of concurrent task workers.
	Workers int
}

type TelegramConfig struct {
	Token              string
	PollTimeoutSeconds int
}

type AnthropicConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int
}

type AgentConfig struct {
	SystemPrompt    string
	MaxSteps        int
	HistoryLimit    int
	AckAfterSeconds int
}

// Load reads configuration from a .env file (if present) and the environment.
func Load() *Config {
	// Best-effort: .env is optional in production/container environments.
	_ = godotenv.Load()

	cfg := &Config{
		Telegram: TelegramConfig{
			Token:              env("TELEGRAM_TOKEN", ""),
			PollTimeoutSeconds: envInt("TELEGRAM_POLL_TIMEOUT_SECONDS", 30),
		},
		Anthropic: AnthropicConfig{
			APIKey:    env("ANTHROPIC_API_KEY", ""),
			BaseURL:   env("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
			Model:     env("ANTHROPIC_MODEL", "claude-opus-4-8"),
			MaxTokens: envInt("ANTHROPIC_MAX_TOKENS", 2000),
		},
		Agent: AgentConfig{
			SystemPrompt:    env("AGENT_SYSTEM_PROMPT", defaultSystemPrompt),
			MaxSteps:        envInt("AGENT_MAX_STEPS", 8),
			HistoryLimit:    envInt("AGENT_HISTORY_LIMIT", 20),
			AckAfterSeconds: envInt("AGENT_ACK_AFTER_SECONDS", 4),
		},
		AllowedUserIDs: parseUserIDs(env("ALLOWED_USER_IDS", "")),
		Workers:        envInt("WORKERS", 2),
	}

	if cfg.Telegram.Token == "" {
		log.Fatal("TELEGRAM_TOKEN must be set")
	}
	if cfg.Anthropic.APIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY must be set")
	}
	if len(cfg.AllowedUserIDs) == 0 {
		log.Fatal("ALLOWED_USER_IDS must list at least one Telegram user id")
	}
	return cfg
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// parseUserIDs parses a comma-separated list of Telegram user ids.
func parseUserIDs(s string) []domain.UserID {
	parts := strings.Split(s, ",")
	out := make([]domain.UserID, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			log.Fatalf("ALLOWED_USER_IDS: invalid id %q", p)
		}
		out = append(out, domain.UserID(n))
	}
	return out
}

const defaultSystemPrompt = `You are a helpful personal assistant operating inside a Telegram bot.
You can answer directly, or use the available tools to solve a task before replying.
Be concise and clear. When a request needs multiple steps, do the work and then
report the result plainly. If you cannot complete something, say so and explain why.`
