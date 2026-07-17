package memory

import (
	"sync"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// LanguageStore is an in-memory per-chat language preference.
type LanguageStore struct {
	mu   sync.RWMutex
	byID map[domain.ChatID]domain.Language
}

func NewLanguageStore() *LanguageStore {
	return &LanguageStore{byID: make(map[domain.ChatID]domain.Language)}
}

func (s *LanguageStore) Get(chatID domain.ChatID) domain.Language {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if l, ok := s.byID[chatID]; ok {
		return l
	}
	return domain.DefaultLanguage
}

func (s *LanguageStore) Set(chatID domain.ChatID, lang domain.Language) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[chatID] = lang
}
