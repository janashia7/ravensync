package connector

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/ravensync/ravensync/internal/ui"
)

type MessageHandler func(ctx context.Context, userID string, text string) (string, error)

type TelegramConnector struct {
	bot     *tgbotapi.BotAPI
	handler MessageHandler
	logger  *slog.Logger
	events  *ui.EventBus
	ownerID string
}

func NewTelegramConnector(token string, ownerID string, handler MessageHandler, logger *slog.Logger, events *ui.EventBus) (*TelegramConnector, error) {
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
		bot:     bot,
		handler: handler,
		logger:  logger,
		events:  events,
		ownerID: ownerID,
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
			if update.Message == nil || update.Message.Text == "" {
				continue
			}
			go tc.handleUpdate(ctx, update)
		}
	}
}

func (tc *TelegramConnector) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	userName := update.Message.From.UserName
	if userName == "" {
		userName = update.Message.From.FirstName
	}
	text := update.Message.Text

	displayName := fmt.Sprintf("tg:%s", userName)

	tc.emit(ui.Event{
		Type:       ui.EventMessageIn,
		UserID:     displayName,
		InternalID: tc.ownerID,
		Message:    text,
	})

	response, err := tc.handler(ctx, tc.ownerID, text)
	if err != nil {
		tc.logger.Error("handler failed", "user_id", tc.ownerID, "error", err)
		response = "Sorry, I encountered an error processing your message."
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
	msg.ReplyToMessageID = update.Message.MessageID

	if _, err := tc.bot.Send(msg); err != nil {
		tc.logger.Error("failed to send reply", "error", err)
		tc.emit(ui.Event{Type: ui.EventError, UserID: displayName, Message: fmt.Sprintf("send failed: %v", err)})
		return
	}

	tc.emit(ui.Event{
		Type:    ui.EventMessageOut,
		UserID:  displayName,
		Message: response,
	})
}

func (tc *TelegramConnector) emit(evt ui.Event) {
	if tc.events != nil {
		tc.events.Emit(evt)
	}
}
