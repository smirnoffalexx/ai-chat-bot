package domain

// Language is a supported UI/response language (ISO 639-1 code).
type Language string

const (
	LangEN Language = "en"
	LangRU Language = "ru"
)

// DefaultLanguage is used until a user picks one.
const DefaultLanguage = LangEN

// Valid reports whether l is a supported language.
func (l Language) Valid() bool {
	return l == LangEN || l == LangRU
}

// MenuButton is one tappable option in an inline menu. Data is an opaque
// callback payload sent back when the button is pressed.
type MenuButton struct {
	Label string
	Data  string
}

// CallbackQuery is a button press from an inline menu.
type CallbackQuery struct {
	ID        string
	ChatID    ChatID
	UserID    UserID
	MessageID int64
	Data      string
}
