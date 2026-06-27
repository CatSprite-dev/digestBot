package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/CatSprite-dev/digestBot/internal/digest"
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
	digest   *digest.Digest
	storage  *storage.Storage
	logger   *slog.Logger
}

func NewBot(token string, resolver ChatResolver, digest *digest.Digest, storage *storage.Storage, logger *slog.Logger) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}
	return &Bot{botAPI: botAPI, resolver: resolver, digest: digest, storage: storage, logger: logger}, nil
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
					b.handleAdd(ctx, update)
				case "remove":
					b.handleRemove(ctx, update)
				case "chats":
					b.handleChats(ctx, update)
				case "digest":
					b.handleDigest(ctx, update)
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (b *Bot) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := b.botAPI.Send(msg); err != nil {
		b.logger.Error("failed to send message", "error", err)
	}
}

func (b *Bot) handleAdd(ctx context.Context, update tgbotapi.Update) {
	args := strings.TrimPrefix(update.Message.CommandArguments(), "@")
	chat, err := b.resolver.ResolveChat(ctx, args)
	if err != nil {
		b.logger.Error("failed to resolve chat", "username", args, "error", err)
		b.send(update.Message.Chat.ID, "❌ Chat not found. Make sure the username is correct.")
		return
	}

	if err := b.storage.UpsertChat(ctx, chat); err != nil {
		b.logger.Error("failed to save chat to storage", "chat_id", chat.ID, "error", err)
		b.send(update.Message.Chat.ID, "❌ Failed to add chat. Please try again.")
		return
	}

	b.send(update.Message.Chat.ID, "✅ Chat added: "+chat.Title)
}

func (b *Bot) handleRemove(ctx context.Context, update tgbotapi.Update) {
	args := strings.TrimPrefix(update.Message.CommandArguments(), "@")
	chat, err := b.resolver.ResolveChat(ctx, args)
	if err != nil {
		b.logger.Error("failed to resolve chat", "username", args, "error", err)
		b.send(update.Message.Chat.ID, "❌ Chat not found. Make sure the username is correct.")
		return
	}

	if err := b.storage.DeleteChat(ctx, chat); err != nil {
		b.logger.Error("failed to delete chat from storage", "chat_id", chat.ID, "error", err)
		b.send(update.Message.Chat.ID, "❌ Failed to remove chat. Please try again.")
		return
	}

	b.send(update.Message.Chat.ID, "✅ Chat removed: "+chat.Title)
}

func (b *Bot) handleChats(ctx context.Context, update tgbotapi.Update) {
	chats, err := b.storage.GetChats(ctx)
	if err != nil {
		b.logger.Error("failed to fetch chats from storage", "error", err)
		b.send(update.Message.Chat.ID, "❌ Failed to load chat list. Please try again.")
		return
	}

	var sb strings.Builder
	sb.WriteString("Tracked chats:\n")
	for _, chat := range chats {
		sb.WriteString("• " + chat.Title + " ")
		if chat.Username != "" {
			sb.WriteString("(" + chat.Username + ")\n")
		} else {
			sb.WriteString("\n")
		}
	}
	b.send(update.Message.Chat.ID, sb.String())
}

func (b *Bot) handleDigest(ctx context.Context, update tgbotapi.Update) {
	args := strings.TrimPrefix(update.Message.CommandArguments(), "@")
	chat, err := b.resolver.ResolveChat(ctx, args)
	if err != nil {
		b.logger.Error("failed to resolve chat", "username", args, "error", err)
		b.send(update.Message.Chat.ID, "❌ Chat not found. Make sure the username is correct.")
		return
	}

	userID := update.Message.From.ID
	cursor, err := b.storage.GetCursor(ctx, userID, chat.ID)
	if err != nil {
		b.logger.Error("failed to get digest cursor", "user_id", userID, "chat_id", chat.ID, "error", err)
		b.send(update.Message.Chat.ID, "❌ Something went wrong. Please try again.")
		return
	}
	messages, err := b.storage.GetMessagesSince(ctx, chat.ID, cursor)
	if err != nil {
		b.logger.Error("failed to fetch messages", "chat_id", chat.ID, "error", err)
		b.send(update.Message.Chat.ID, "❌ Failed to load messages. Please try again.")
		return
	}

	if len(messages) == 0 {
		b.send(update.Message.Chat.ID, "No new messages since last digest.")
		return
	}

	messages, truncated := digest.LimitMessages(messages, 100_000)
	if truncated {
		b.send(update.Message.Chat.ID, fmt.Sprintf("⚠️ Показаны последние %d сообщений — слишком много для одного дайджеста.", len(messages)))
	}

	digestText, err := b.digest.Generate(ctx, messages)
	if err != nil {
		b.logger.Error("failed to generate digest", "error", err)
		b.send(update.Message.Chat.ID, "❌ Failed to generate digest. Please try again.")
		return
	}

	if err := b.storage.SetCursor(ctx, userID, chat.ID, time.Now()); err != nil {
		b.logger.Error("failed to update cursor", "user_id", userID, "chat_id", chat.ID, "error", err)
	}

	b.send(update.Message.Chat.ID, digestText)
}
