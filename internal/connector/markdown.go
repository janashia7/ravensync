package connector

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var mdV2Replacer = strings.NewReplacer(
	"_", "\\_",
	"*", "\\*",
	"[", "\\[",
	"]", "\\]",
	"(", "\\(",
	")", "\\)",
	"~", "\\~",
	"`", "\\`",
	">", "\\>",
	"#", "\\#",
	"+", "\\+",
	"-", "\\-",
	"=", "\\=",
	"|", "\\|",
	"{", "\\{",
	"}", "\\}",
	".", "\\.",
	"!", "\\!",
)

func escapeMarkdownV2(text string) string {
	return mdV2Replacer.Replace(text)
}

func sendMarkdownV2(bot *tgbotapi.BotAPI, chatID int64, replyTo int, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, escapeMarkdownV2(text))
	msg.ReplyToMessageID = replyTo
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	sent, err := bot.Send(msg)
	if err != nil {
		plain := tgbotapi.NewMessage(chatID, text)
		plain.ReplyToMessageID = replyTo
		return bot.Send(plain)
	}
	return sent, nil
}

func editMarkdownV2(bot *tgbotapi.BotAPI, chatID int64, msgID int, text string) error {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, escapeMarkdownV2(text))
	edit.ParseMode = tgbotapi.ModeMarkdownV2

	_, err := bot.Send(edit)
	if err != nil {
		plainEdit := tgbotapi.NewEditMessageText(chatID, msgID, text)
		_, err = bot.Send(plainEdit)
	}
	return err
}
