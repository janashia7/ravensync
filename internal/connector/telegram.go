package connector

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/ravensync/ravensync/internal/agent"
	"github.com/ravensync/ravensync/internal/llm"
	"github.com/ravensync/ravensync/internal/ui"
)

const maxTelegramImageBytes = 10 * 1024 * 1024

type TelegramConnector struct {
	bot              *tgbotapi.BotAPI
	agent            *agent.Agent
	logger           *slog.Logger
	events           *ui.EventBus
	allowedUsers     []int64
	allowedUsernames []string // normalized: lowercase, no @
	botID            int64
	startTime        time.Time

	callbackMu    sync.Mutex
	callbackChans map[int64]chan string // chatID -> pending callback response
}

func NewTelegramConnector(token string, allowedUsers []int64, allowedUsernames []string, ag *agent.Agent, logger *slog.Logger, events *ui.EventBus) (*TelegramConnector, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	logger.Info("telegram bot authorized", "username", bot.Self.UserName)

	if events != nil {
		events.Emit(ui.Event{
			Type:    ui.EventInfo,
			Message: fmt.Sprintf("telegram bot authorized as @%s", bot.Self.UserName),
		})
	}

	return &TelegramConnector{
		bot:              bot,
		agent:            ag,
		logger:           logger,
		events:           events,
		allowedUsers:     allowedUsers,
		allowedUsernames: normalizeAllowedUsernames(allowedUsernames),
		botID:            bot.Self.ID,
		startTime:        time.Now(),
		callbackChans:    make(map[int64]chan string),
	}, nil
}

func (tc *TelegramConnector) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tc.bot.GetUpdatesChan(u)

	tc.logger.Info("listening for telegram messages")
	tc.emit(ui.Event{Type: ui.EventInfo, Message: "listening for telegram messages..."})

	for {
		select {
		case <-ctx.Done():
			tc.bot.StopReceivingUpdates()
			return ctx.Err()
		case update := <-updates:
			if update.CallbackQuery != nil {
				cq := update.CallbackQuery
				if cq.From != nil && !tc.isAllowed(cq.From) {
					continue
				}
				go tc.handleCallbackQuery(cq)
				continue
			}
			if update.Message == nil {
				continue
			}
			if update.Message.From != nil && update.Message.From.ID == tc.botID {
				continue
			}
			if !tc.isAllowed(update.Message.From) {
				tc.logger.Debug("dropping message from non-allowed user",
					"user_id", update.Message.From.ID,
					"chat_id", update.Message.Chat.ID,
				)
				continue
			}
			go tc.handleUpdate(ctx, update)
		}
	}
}

func (tc *TelegramConnector) isAllowed(from *tgbotapi.User) bool {
	if from == nil {
		return false
	}
	if len(tc.allowedUsers) == 0 && len(tc.allowedUsernames) == 0 {
		return true
	}
	for _, id := range tc.allowedUsers {
		if id == from.ID {
			return true
		}
	}
	if from.UserName != "" {
		u := normalizeTelegramUsername(from.UserName)
		for _, allowed := range tc.allowedUsernames {
			if allowed == u {
				return true
			}
		}
	}
	return false
}

func normalizeTelegramUsername(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	return strings.ToLower(s)
}

func normalizeAllowedUsernames(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{})
	for _, s := range list {
		n := normalizeTelegramUsername(s)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func (tc *TelegramConnector) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	msg := update.Message
	fromID := msg.From.ID
	userID := fmt.Sprintf("tg:%d", fromID)
	chatID := msg.Chat.ID

	userName := msg.From.UserName
	if userName == "" {
		userName = msg.From.FirstName
	}
	displayName := fmt.Sprintf("tg:%s", userName)

	text := tc.buildUserText(msg)
	images := tc.downloadPhotoImages(ctx, msg)
	if text == "" && len(images) == 0 {
		return
	}

	if cmd := ParseCommand(text); cmd != nil {
		cc := CommandContext{
			UserID:    userID,
			Store:     tc.agent.Store(),
			Metrics:   tc.agent.Metrics(),
			StartTime: tc.startTime,
		}
		response := DispatchCommand(cmd, cc)
		tc.sendReply(chatID, msg.MessageID, response)
		return
	}

	tc.emit(ui.Event{
		Type:       ui.EventMessageIn,
		UserID:     displayName,
		InternalID: userID,
		Message:    text,
	})

	tc.sendTyping(chatID)

	typingCtx, cancelTyping := context.WithCancel(ctx)
	go tc.keepTyping(typingCtx, chatID)

	partials, done := tc.agent.HandleMessageStream(ctx, userID, text, images)

	var sentMsgID int
	var lastEdit time.Time

	for partial := range partials {
		if sentMsgID == 0 {
			sent, err := tc.sendPlain(chatID, msg.MessageID, partial+"...")
			if err == nil {
				sentMsgID = sent.MessageID
				lastEdit = time.Now()
			}
			continue
		}

		if time.Since(lastEdit) < 300*time.Millisecond {
			continue
		}

		_ = editMarkdownV2(tc.bot, chatID, sentMsgID, partial+"...")
		lastEdit = time.Now()
	}

	cancelTyping()

	result := <-done

	if result.Err != nil {
		tc.logger.Error("handler failed", "user_id", userID, "error", result.Err)
		errorText := "Sorry, I encountered an error processing your message."
		if sentMsgID != 0 {
			_ = editMarkdownV2(tc.bot, chatID, sentMsgID, errorText)
		} else {
			tc.sendReply(chatID, msg.MessageID, errorText)
		}
		return
	}

	response := result.FullResponse
	if response == "" {
		response = "(empty response)"
	}

	responseText, choices := parseChoices(response)

	if sentMsgID != 0 {
		if len(choices) > 0 {
			tc.editWithKeyboard(chatID, sentMsgID, responseText, choices)
		} else {
			_ = editMarkdownV2(tc.bot, chatID, sentMsgID, responseText)
		}
	} else {
		if len(choices) > 0 {
			tc.sendWithKeyboard(chatID, msg.MessageID, responseText, choices)
		} else {
			if _, err := sendMarkdownV2(tc.bot, chatID, msg.MessageID, responseText); err != nil {
				tc.logger.Debug("send markdown reply failed", "error", err)
			}
		}
	}

	tc.emit(ui.Event{
		Type:    ui.EventMessageOut,
		UserID:  displayName,
		Message: responseText,
	})
}

func (tc *TelegramConnector) buildUserText(msg *tgbotapi.Message) string {
	var parts []string

	if msg.ReplyToMessage != nil {
		quoted := msg.ReplyToMessage.Text
		if quoted == "" {
			quoted = msg.ReplyToMessage.Caption
		}
		if quoted != "" {
			if len(quoted) > 200 {
				quoted = quoted[:200] + "..."
			}
			parts = append(parts, fmt.Sprintf("[Replying to: %s]", quoted))
		}
	}

	if msg.Caption != "" {
		parts = append(parts, msg.Caption)
	}

	if msg.Document != nil {
		doc := msg.Document
		content := tc.downloadTextFile(doc.FileID, doc.MimeType)
		if content != "" {
			if len(content) > 2000 {
				content = content[:2000] + "\n...(truncated)"
			}
			parts = append(parts, fmt.Sprintf("[Document: %s]\n%s", doc.FileName, content))
		} else {
			parts = append(parts, fmt.Sprintf("[Document: %s]", doc.FileName))
		}
	}

	if msg.Text != "" {
		parts = append(parts, msg.Text)
	}

	return strings.Join(parts, "\n")
}

func (tc *TelegramConnector) downloadPhotoImages(ctx context.Context, msg *tgbotapi.Message) []llm.ImagePart {
	if len(msg.Photo) == 0 {
		return nil
	}
	best := msg.Photo[len(msg.Photo)-1]
	url := tc.getFileURL(best.FileID)
	if url == "" {
		return nil
	}
	data, mime, err := tc.downloadImage(ctx, url)
	if err != nil {
		tc.logger.Warn("photo download failed", "error", err)
		return nil
	}
	return []llm.ImagePart{{MIME: mime, Data: data}}
}

func (tc *TelegramConnector) getFileURL(fileID string) string {
	file, err := tc.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		tc.logger.Debug("failed to get file", "error", err)
		return ""
	}
	return file.Link(tc.bot.Token)
}

func contentTypeMIME(ct string) string {
	ct = strings.TrimSpace(strings.Split(ct, ";")[0])
	if strings.HasPrefix(ct, "image/") {
		return ct
	}
	return "image/jpeg"
}

func (tc *TelegramConnector) downloadImage(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			tc.logger.Debug("close http body", "error", cerr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.ContentLength > maxTelegramImageBytes {
		return nil, "", fmt.Errorf("image too large")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTelegramImageBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > maxTelegramImageBytes {
		return nil, "", fmt.Errorf("image too large")
	}
	mime := contentTypeMIME(resp.Header.Get("Content-Type"))
	return body, mime, nil
}

func (tc *TelegramConnector) downloadTextFile(fileID, mimeType string) string {
	if !strings.HasPrefix(mimeType, "text/") &&
		mimeType != "application/json" &&
		mimeType != "application/xml" {
		return ""
	}
	url := tc.getFileURL(fileID)
	if url == "" {
		return ""
	}
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return ""
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			tc.logger.Debug("close http body", "error", cerr)
		}
	}()
	if resp.ContentLength > 1024*1024 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return ""
	}
	return string(body)
}

func (tc *TelegramConnector) sendTyping(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, _ = tc.bot.Send(action)
}

func (tc *TelegramConnector) keepTyping(ctx context.Context, chatID int64) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tc.sendTyping(chatID)
		}
	}
}

func (tc *TelegramConnector) sendReply(chatID int64, replyTo int, text string) {
	if _, err := sendMarkdownV2(tc.bot, chatID, replyTo, text); err != nil {
		tc.logger.Debug("send reply failed", "error", err)
	}
}

func (tc *TelegramConnector) sendPlain(chatID int64, replyTo int, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyTo
	return tc.bot.Send(msg)
}

var choicePattern = regexp.MustCompile(`\[CHOOSE:\s*(.+?)\]\s*$`)

func parseChoices(response string) (string, []string) {
	match := choicePattern.FindStringSubmatch(response)
	if match == nil {
		return response, nil
	}
	text := strings.TrimSpace(response[:len(response)-len(match[0])])
	raw := strings.Split(match[1], "|")
	choices := make([]string, 0, len(raw))
	for _, c := range raw {
		c = strings.TrimSpace(c)
		if c != "" {
			choices = append(choices, c)
		}
	}
	if len(choices) < 2 {
		return response, nil
	}
	return text, choices
}

func (tc *TelegramConnector) sendWithKeyboard(chatID int64, replyTo int, text string, choices []string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyTo
	msg.ReplyMarkup = buildInlineKeyboard(choices)
	_, _ = tc.bot.Send(msg)
}

func (tc *TelegramConnector) editWithKeyboard(chatID int64, msgID int, text string, choices []string) {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	kb := buildInlineKeyboard(choices)
	edit.ReplyMarkup = &kb
	_, _ = tc.bot.Send(edit)
}

func buildInlineKeyboard(choices []string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range choices {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(c, "choice:"+c),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (tc *TelegramConnector) handleCallbackQuery(cq *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cq.ID, "")
	_, _ = tc.bot.Request(callback)

	if !strings.HasPrefix(cq.Data, "choice:") {
		return
	}

	chosen := strings.TrimPrefix(cq.Data, "choice:")

	if cq.Message != nil {
		edit := tgbotapi.NewEditMessageReplyMarkup(
			cq.Message.Chat.ID,
			cq.Message.MessageID,
			tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}},
		)
		_, _ = tc.bot.Send(edit)

		text := cq.Message.Text + "\n\nYou chose: " + chosen
		editText := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, text)
		_, _ = tc.bot.Send(editText)
	}

	tc.callbackMu.Lock()
	if cq.Message != nil {
		if ch, ok := tc.callbackChans[cq.Message.Chat.ID]; ok {
			select {
			case ch <- chosen:
			default:
			}
		}
	}
	tc.callbackMu.Unlock()
}

func (tc *TelegramConnector) emit(evt ui.Event) {
	if tc.events != nil {
		tc.events.Emit(evt)
	}
}
