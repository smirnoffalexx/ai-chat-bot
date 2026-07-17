// Package telegram implements the inbound poller and the outbound port.Sender
// against the Telegram Bot API using only the standard library.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
	"github.com/smirnoffalexx/ai-chat-bot/internal/port"
)

const (
	apiBase     = "https://api.telegram.org/bot"
	maxMsgRunes = 4096
)

// Client talks to the Telegram Bot API.
type Client struct {
	http        *http.Client
	token       string
	pollTimeout int
	log         *slog.Logger
}

func New(token string, pollTimeoutSeconds int, log *slog.Logger) *Client {
	return &Client{
		// Timeout must exceed the long-poll timeout.
		http:        &http.Client{Timeout: time.Duration(pollTimeoutSeconds+15) * time.Second},
		token:       token,
		pollTimeout: pollTimeoutSeconds,
		log:         log,
	}
}

var _ port.Sender = (*Client)(nil)

// Handlers receives dispatched updates. Any field may be nil.
type Handlers struct {
	Message  func(ctx context.Context, m domain.IncomingMessage)
	Callback func(ctx context.Context, cb domain.CallbackQuery)
}

// Poll long-polls getUpdates and dispatches updates until ctx is cancelled.
func (c *Client) Poll(ctx context.Context, h Handlers) {
	var offset int64
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := c.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("getUpdates", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second): // back off before retrying
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			switch {
			case u.CallbackQuery != nil && h.Callback != nil:
				h.Callback(ctx, toCallback(u.CallbackQuery))
			case u.Message != nil && u.Message.Text != "" && h.Message != nil:
				h.Message(ctx, toIncoming(u.Message))
			}
		}
	}
}

func toCallback(cb *tgCallbackQuery) domain.CallbackQuery {
	q := domain.CallbackQuery{
		ID:     cb.ID,
		UserID: domain.UserID(cb.From.ID),
		Data:   cb.Data,
	}
	if cb.Message != nil {
		q.ChatID = domain.ChatID(cb.Message.Chat.ID)
		q.MessageID = cb.Message.MessageID
	}
	return q
}

func toIncoming(m *tgMessage) domain.IncomingMessage {
	name := m.From.Username
	if name == "" {
		name = m.From.FirstName
	}
	return domain.IncomingMessage{
		ChatID:    domain.ChatID(m.Chat.ID),
		UserID:    domain.UserID(m.From.ID),
		UserName:  name,
		MessageID: m.MessageID,
		Text:      m.Text,
	}
}

func (c *Client) getUpdates(ctx context.Context, offset int64) ([]tgUpdate, error) {
	q := url.Values{}
	q.Set("timeout", strconv.Itoa(c.pollTimeout))
	q.Set("offset", strconv.FormatInt(offset, 10))
	q.Set("allowed_updates", `["message","callback_query"]`)

	var resp struct {
		OK          bool       `json:"ok"`
		Description string     `json:"description"`
		Result      []tgUpdate `json:"result"`
	}
	if err := c.call(ctx, "getUpdates", q, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram: %s", resp.Description)
	}
	return resp.Result, nil
}

// SendText sends text, splitting into chunks that respect Telegram's limit.
func (c *Client) SendText(ctx context.Context, chatID domain.ChatID, text string) error {
	for _, chunk := range splitMessage(text, maxMsgRunes) {
		q := url.Values{}
		q.Set("chat_id", strconv.FormatInt(int64(chatID), 10))
		q.Set("text", chunk)
		var resp struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		if err := c.call(ctx, "sendMessage", q, &resp); err != nil {
			return err
		}
		if !resp.OK {
			return fmt.Errorf("telegram sendMessage: %s", resp.Description)
		}
	}
	return nil
}

// SendMenu sends text with a single row of inline buttons.
func (c *Client) SendMenu(ctx context.Context, chatID domain.ChatID, text string, buttons []domain.MenuButton) error {
	markup, err := json.Marshal(inlineKeyboard(buttons))
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("chat_id", strconv.FormatInt(int64(chatID), 10))
	q.Set("text", text)
	q.Set("reply_markup", string(markup))
	var resp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := c.call(ctx, "sendMessage", q, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("telegram sendMessage: %s", resp.Description)
	}
	return nil
}

// AnswerCallback acknowledges a button press so the client stops its spinner.
func (c *Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	q := url.Values{}
	q.Set("callback_query_id", callbackID)
	if text != "" {
		q.Set("text", text)
	}
	var resp struct {
		OK bool `json:"ok"`
	}
	return c.call(ctx, "answerCallbackQuery", q, &resp)
}

// inlineKeyboard puts all buttons in a single row.
func inlineKeyboard(buttons []domain.MenuButton) map[string]any {
	row := make([]map[string]string, 0, len(buttons))
	for _, b := range buttons {
		row = append(row, map[string]string{"text": b.Label, "callback_data": b.Data})
	}
	return map[string]any{"inline_keyboard": [][]map[string]string{row}}
}

// SendTyping shows the "typing…" indicator (best effort).
func (c *Client) SendTyping(ctx context.Context, chatID domain.ChatID) error {
	q := url.Values{}
	q.Set("chat_id", strconv.FormatInt(int64(chatID), 10))
	q.Set("action", "typing")
	var resp struct {
		OK bool `json:"ok"`
	}
	return c.call(ctx, "sendChatAction", q, &resp)
}

// call performs a POST with form values and decodes the JSON response.
func (c *Client) call(ctx context.Context, method string, form url.Values, out any) error {
	endpoint := apiBase + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s response (status %d): %w", method, resp.StatusCode, err)
	}
	return nil
}

// splitMessage splits s into chunks of at most limit runes, preferring to break
// on newlines. It never splits a multi-byte rune.
func splitMessage(s string, limit int) []string {
	runes := []rune(s)
	if len(runes) <= limit {
		return []string{s}
	}
	var chunks []string
	for len(runes) > 0 {
		end := limit
		if end > len(runes) {
			end = len(runes)
		}
		// Try to break on the last newline within the window.
		if end < len(runes) {
			for i := end - 1; i > limit/2; i-- {
				if runes[i] == '\n' {
					end = i + 1
					break
				}
			}
		}
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

// --- Telegram wire types ---

type tgUpdate struct {
	UpdateID      int64            `json:"update_id"`
	Message       *tgMessage       `json:"message"`
	CallbackQuery *tgCallbackQuery `json:"callback_query"`
}

type tgCallbackQuery struct {
	ID   string `json:"id"`
	Data string `json:"data"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Message *tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	From      struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	} `json:"from"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}
