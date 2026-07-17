// Package i18n holds localized bot strings and a lookup helper. Add a language
// by extending the messages table; missing keys fall back to English.
package i18n

import "github.com/smirnoffalexx/ai-chat-bot/internal/domain"

// Message keys. Values may contain fmt verbs where noted.
const (
	Help            = "help"
	Unauthorized    = "unauthorized"
	Working         = "working"
	Cleared         = "cleared"
	UnknownCommand  = "unknown_command" // %q command
	AcceptFailed    = "accept_failed"
	ErrorPrefix     = "error_prefix" // %s error
	NothingToReport = "nothing_to_report"
	ChooseLanguage  = "choose_language"
	LanguageSet     = "language_set"
)

// T returns the message for key in lang, falling back to English, then the key.
func T(lang domain.Language, key string) string {
	if m, ok := messages[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := messages[domain.LangEN][key]; ok {
		return s
	}
	return key
}

// ReplyDirective is a system instruction telling the model which language to
// answer in, so responses match the user's chosen UI language.
func ReplyDirective(lang domain.Language) string {
	switch lang {
	case domain.LangRU:
		return "Всегда отвечай пользователю на русском языке."
	default:
		return "Always reply to the user in English."
	}
}

// LanguageButtons are the fixed-label options shown by the /language menu.
func LanguageButtons() []domain.MenuButton {
	return []domain.MenuButton{
		{Label: "🇬🇧 English", Data: "lang:en"},
		{Label: "🇷🇺 Русский", Data: "lang:ru"},
	}
}

var messages = map[domain.Language]map[string]string{
	domain.LangEN: {
		Help: `Hi! I'm your personal AI assistant.

Just send me a message and I'll answer, or I'll work on the task and reply when it's done.

Commands:
/help     – show this message
/language – switch language (English / Russian)
/reset    – clear our conversation memory`,
		Unauthorized:    "You are not authorized to use this bot.",
		Working:         "🛠 Working on it — I'll reply as soon as it's done.",
		Cleared:         "Conversation memory cleared.",
		UnknownCommand:  "Unknown command %q. Try /help.",
		AcceptFailed:    "Sorry, I couldn't accept that right now. Please try again.",
		ErrorPrefix:     "Sorry — I hit an error working on that:\n%s",
		NothingToReport: "(done, but I have nothing to report)",
		ChooseLanguage:  "Choose your language:",
		LanguageSet:     "✅ Language set to English.",
	},
	domain.LangRU: {
		Help: `Привет! Я ваш персональный ИИ-ассистент.

Просто напишите мне — я отвечу сразу или возьмусь за задачу и пришлю ответ, когда закончу.

Команды:
/help     – показать это сообщение
/language – сменить язык (английский / русский)
/reset    – очистить память разговора`,
		Unauthorized:    "У вас нет доступа к этому боту.",
		Working:         "🛠 Работаю над этим — отвечу, как только будет готово.",
		Cleared:         "Память разговора очищена.",
		UnknownCommand:  "Неизвестная команда %q. Наберите /help.",
		AcceptFailed:    "Извините, сейчас не удалось принять запрос. Попробуйте ещё раз.",
		ErrorPrefix:     "Извините — при выполнении возникла ошибка:\n%s",
		NothingToReport: "(готово, но сообщить нечего)",
		ChooseLanguage:  "Выберите язык:",
		LanguageSet:     "✅ Язык переключён на русский.",
	},
}
