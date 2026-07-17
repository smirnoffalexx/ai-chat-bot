package domain

// IncomingMessage is a message received from a user via the messaging platform.
type IncomingMessage struct {
	ChatID    ChatID
	UserID    UserID
	UserName  string // human-readable, for logging/prompts only
	MessageID int64
	Text      string
}
