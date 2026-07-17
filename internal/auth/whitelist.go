// Package auth implements user authorization for the bot.
package auth

import "github.com/smirnoffalexx/ai-chat-bot/internal/domain"

// Whitelist allows only a fixed set of user IDs.
type Whitelist struct {
	allowed map[domain.UserID]struct{}
}

func NewWhitelist(ids []domain.UserID) *Whitelist {
	m := make(map[domain.UserID]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return &Whitelist{allowed: m}
}

func (w *Whitelist) Allowed(id domain.UserID) bool {
	_, ok := w.allowed[id]
	return ok
}
