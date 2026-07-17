package usecase

import (
	"context"
	"testing"

	"github.com/smirnoffalexx/ai-chat-bot/internal/adapter/memory"
	"github.com/smirnoffalexx/ai-chat-bot/internal/auth"
	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// fakeSender records outbound calls for assertions.
type fakeSender struct {
	texts          []string
	menuButtons    []domain.MenuButton
	answered       bool
	answeredWith   string
	lastMenuChatID domain.ChatID
}

func (f *fakeSender) SendText(_ context.Context, _ domain.ChatID, text string) error {
	f.texts = append(f.texts, text)
	return nil
}
func (f *fakeSender) SendTyping(_ context.Context, _ domain.ChatID) error { return nil }
func (f *fakeSender) SendMenu(_ context.Context, chatID domain.ChatID, _ string, b []domain.MenuButton) error {
	f.lastMenuChatID = chatID
	f.menuButtons = b
	return nil
}
func (f *fakeSender) AnswerCallback(_ context.Context, _, text string) error {
	f.answered = true
	f.answeredWith = text
	return nil
}

func newTestHandler(sender *fakeSender, allowed domain.UserID) (*Handler, *memory.LanguageStore) {
	langs := memory.NewLanguageStore()
	h := NewHandler(
		auth.NewWhitelist([]domain.UserID{allowed}),
		memory.NewQueue(4),
		memory.NewTaskRepository(),
		memory.NewHistory(),
		langs,
		sender,
		testLogger(),
	)
	return h, langs
}

func TestLanguageCommandShowsMenu(t *testing.T) {
	sender := &fakeSender{}
	h, _ := newTestHandler(sender, 1)

	h.Handle(context.Background(), domain.IncomingMessage{
		ChatID: 1, UserID: 1, Text: "/language",
	})

	if len(sender.menuButtons) != 2 {
		t.Fatalf("expected 2 language buttons, got %d", len(sender.menuButtons))
	}
	if sender.menuButtons[1].Data != "lang:ru" {
		t.Fatalf("unexpected button data: %q", sender.menuButtons[1].Data)
	}
}

func TestCallbackSetsLanguage(t *testing.T) {
	sender := &fakeSender{}
	h, langs := newTestHandler(sender, 1)

	h.HandleCallback(context.Background(), domain.CallbackQuery{
		ID: "cb1", ChatID: 1, UserID: 1, Data: "lang:ru",
	})

	if got := langs.Get(1); got != domain.LangRU {
		t.Fatalf("language = %q, want ru", got)
	}
	if !sender.answered {
		t.Fatal("callback was not acknowledged")
	}
	// Confirmation should be sent in Russian.
	if len(sender.texts) != 1 || sender.texts[0] != "✅ Язык переключён на русский." {
		t.Fatalf("unexpected confirmation texts: %v", sender.texts)
	}
}

func TestUnauthorizedCallbackIsRejected(t *testing.T) {
	sender := &fakeSender{}
	h, langs := newTestHandler(sender, 1)

	h.HandleCallback(context.Background(), domain.CallbackQuery{
		ID: "cb1", ChatID: 2, UserID: 999, Data: "lang:ru",
	})

	if langs.Get(2) != domain.DefaultLanguage {
		t.Fatal("language must not change for unauthorized user")
	}
	if !sender.answered {
		t.Fatal("callback should still be acknowledged")
	}
}
