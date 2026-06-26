package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/CatSprite-dev/digestBot/internal/model"
	"github.com/CatSprite-dev/digestBot/internal/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ChatResolver interface {
	ResolveChat(ctx context.Context, username string) (model.Chat, error)
}

type Bot struct {
	botAPI   *tgbotapi.BotAPI
	resolver ChatResolver
	storage  *storage.Storage
	logger   *slog.Logger
}

func NewBot(token string, storage *storage.Storage, logger *slog.Logger, resolver ChatResolver) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}
	return &Bot{botAPI: botAPI, storage: storage, logger: logger, resolver: resolver}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	// настраиваем получение апдейтов
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// получаем канал апдейтов
	updates := b.botAPI.GetUpdatesChan(u)

	// читаем апдейты пока не отменят контекст
	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "add":
					args := strings.TrimPrefix(update.Message.CommandArguments(), "@")
					chat, err := b.resolver.ResolveChat(ctx, args)
					if err != nil {
						b.logger.Error("failed to resolve chat", "error", err)
						continue
					}

					err = b.storage.UpsertChat(ctx, chat)
					if err != nil {
						b.logger.Error("failed to upsert chat", "error", err)
						continue
					}
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Chat added successfully")
					if _, err := b.botAPI.Send(msg); err != nil {
						b.logger.Error("failed to send message", "error", err)
					}
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}
