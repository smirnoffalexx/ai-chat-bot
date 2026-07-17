package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/i18n"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

// Handler is the inbound entry point: it authorizes users, handles commands and
// menu callbacks, and enqueues chat messages as background tasks.
type Handler struct {
	auth    port.Authorizer
	queue   port.Queue
	repo    port.TaskRepository
	history port.History
	lang    port.LanguageStore
	sender  port.Sender
	log     *slog.Logger
}

func NewHandler(
	auth port.Authorizer,
	queue port.Queue,
	repo port.TaskRepository,
	history port.History,
	lang port.LanguageStore,
	sender port.Sender,
	log *slog.Logger,
) *Handler {
	return &Handler{auth: auth, queue: queue, repo: repo, history: history, lang: lang, sender: sender, log: log}
}

// Handle processes one incoming message. It never blocks on the LLM: the actual
// work is queued and answered by a worker.
func (h *Handler) Handle(ctx context.Context, m domain.IncomingMessage) {
	if !h.auth.Allowed(m.UserID) {
		h.log.Warn("rejected unauthorized user", "user_id", m.UserID, "name", m.UserName)
		_ = h.sender.SendText(ctx, m.ChatID, i18n.T(h.lang.Get(m.ChatID), i18n.Unauthorized))
		return
	}

	text := strings.TrimSpace(m.Text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		h.handleCommand(ctx, m, text)
		return
	}

	task := domain.Task{
		ID:        newID(),
		ChatID:    m.ChatID,
		UserID:    m.UserID,
		UserName:  m.UserName,
		Prompt:    text,
		Status:    domain.TaskQueued,
		CreatedAt: time.Now(),
	}
	if err := h.repo.Save(ctx, task); err != nil {
		h.log.Error("save task", "err", err)
	}
	if err := h.queue.Enqueue(ctx, task); err != nil {
		h.log.Error("enqueue task", "err", err)
		_ = h.sender.SendText(ctx, m.ChatID, i18n.T(h.lang.Get(m.ChatID), i18n.AcceptFailed))
		return
	}
	// Signal receipt immediately; the worker sends the real answer later.
	_ = h.sender.SendTyping(ctx, m.ChatID)
}

func (h *Handler) handleCommand(ctx context.Context, m domain.IncomingMessage, text string) {
	lang := h.lang.Get(m.ChatID)
	cmd := strings.Fields(text)[0]
	switch cmd {
	case "/start", "/help":
		_ = h.sender.SendText(ctx, m.ChatID, i18n.T(lang, i18n.Help))
	case "/language", "/lang":
		_ = h.sender.SendMenu(ctx, m.ChatID, i18n.T(lang, i18n.ChooseLanguage), i18n.LanguageButtons())
	case "/reset", "/clear":
		h.history.Reset(m.ChatID)
		_ = h.sender.SendText(ctx, m.ChatID, i18n.T(lang, i18n.Cleared))
	default:
		_ = h.sender.SendText(ctx, m.ChatID, fmt.Sprintf(i18n.T(lang, i18n.UnknownCommand), cmd))
	}
}

// HandleCallback processes an inline-button press (currently: language choice).
func (h *Handler) HandleCallback(ctx context.Context, cb domain.CallbackQuery) {
	if !h.auth.Allowed(cb.UserID) {
		_ = h.sender.AnswerCallback(ctx, cb.ID, i18n.T(domain.DefaultLanguage, i18n.Unauthorized))
		return
	}

	if code, ok := strings.CutPrefix(cb.Data, "lang:"); ok {
		lang := domain.Language(code)
		if !lang.Valid() {
			lang = domain.DefaultLanguage
		}
		h.lang.Set(cb.ChatID, lang)
		_ = h.sender.AnswerCallback(ctx, cb.ID, "")
		_ = h.sender.SendText(ctx, cb.ChatID, i18n.T(lang, i18n.LanguageSet))
		return
	}

	// Unknown callback: acknowledge so the client's spinner stops.
	_ = h.sender.AnswerCallback(ctx, cb.ID, "")
}

// newID returns a random 128-bit hex identifier for a task.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
