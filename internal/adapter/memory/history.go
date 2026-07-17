package memory

import (
	"sync"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// History is a per-chat conversation store guarded by a mutex.
type History struct {
	mu   sync.Mutex
	byID map[domain.ChatID][]domain.Message
}

func NewHistory() *History {
	return &History{byID: make(map[domain.ChatID][]domain.Message)}
}

func (h *History) Append(chatID domain.ChatID, msgs ...domain.Message) {
	if len(msgs) == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byID[chatID] = append(h.byID[chatID], msgs...)
}

func (h *History) Get(chatID domain.ChatID) []domain.Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	src := h.byID[chatID]
	// Return a copy so callers cannot mutate stored state.
	out := make([]domain.Message, len(src))
	copy(out, src)
	return out
}

func (h *History) Reset(chatID domain.ChatID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.byID, chatID)
}
