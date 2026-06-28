package userbot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CatSprite-dev/digestBot/internal/model"
	"github.com/CatSprite-dev/digestBot/internal/storage"
	"github.com/gotd/td/tg"
)

type Userbot struct {
	client     *tg.Client
	storage    *storage.Storage
	logger     *slog.Logger
	dispatcher tg.UpdateDispatcher
}

func NewUserbot(client *tg.Client, storage *storage.Storage, logger *slog.Logger, dispatcher tg.UpdateDispatcher) *Userbot {
	return &Userbot{client: client, storage: storage, logger: logger, dispatcher: dispatcher}
}

func (ub *Userbot) Listen() {
	ub.dispatcher.OnNewMessage(ub.handleNewMessage)
	ub.dispatcher.OnNewChannelMessage(ub.handleNewChannelMessage)
}

func (ub *Userbot) handleNewMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateNewMessage) error {
	msg, ok := update.Message.(*tg.Message)
	if !ok {
		return nil
	}
	return ub.processMessage(ctx, entities, msg)
}

func (ub *Userbot) handleNewChannelMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateNewChannelMessage) error {
	msg, ok := update.Message.(*tg.Message)
	if !ok {
		return nil
	}
	return ub.processMessage(ctx, entities, msg)
}

func (ub *Userbot) processMessage(ctx context.Context, entities tg.Entities, msg *tg.Message) error {
	if !isValidMessage(msg) {
		return nil
	}
	var chatID int64
	switch peer := msg.PeerID.(type) {
	case *tg.PeerChannel:
		chatID = peer.ChannelID
	case *tg.PeerChat:
		chatID = peer.ChatID
	default:
		return nil
	}

	exist, err := ub.storage.ChatExists(ctx, chatID)
	if err != nil {
		ub.logger.Error("failed to check chat existence", "chat_id", chatID, "error", err)
		return nil
	}
	if !exist {
		return nil
	}

	sender := extractSender(entities.Users, msg)
	err = ub.buildAndSave(ctx, chatID, sender, msg)
	if err != nil {
		ub.logger.Error("failed to save message", "chat_id", chatID, "error", err)
	}
	ub.logger.Debug("message received", "chat_id", chatID, "text", msg.Message)
	return nil
}

func (ub *Userbot) ResolveChat(ctx context.Context, username string) (model.Chat, error) {
	resolved, err := ub.client.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return model.Chat{}, fmt.Errorf("resolve username: %w", err)
	}
	if len(resolved.Chats) == 0 {
		return model.Chat{}, fmt.Errorf("chat not found")
	}

	switch chat := resolved.Chats[0].(type) {
	case *tg.Channel:
		return model.Chat{
			ID:         chat.ID,
			Username:   chat.Username,
			Title:      chat.Title,
			AccessHash: chat.AccessHash,
		}, nil
	case *tg.Chat:
		return model.Chat{
			ID:    chat.ID,
			Title: chat.Title,
		}, nil
	default:
		return model.Chat{}, fmt.Errorf("unknown chat type")
	}
}

func (ub *Userbot) LoadHistory(ctx context.Context, chat model.Chat) error {
	const (
		batchSize = 100
		maxChars  = 50_000
		maxAge    = 7 * 24 * time.Hour
	)

	edge := time.Now().Add(-maxAge)
	peer := tg.InputPeerChannel{ChannelID: chat.ID, AccessHash: chat.AccessHash}

	offsetID := 0
	totalChars := 0
	savedCount := 0

	for {
		chunk, err := ub.client.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     &peer,
			Limit:    batchSize,
			OffsetID: offsetID,
		})
		if err != nil {
			return fmt.Errorf("get history (chat %d): %w", chat.ID, err)
		}

		messages, ok := chunk.(*tg.MessagesChannelMessages)
		if !ok {
			ub.logger.Warn("unexpected history type", "chat_id", chat.ID, "type", fmt.Sprintf("%T", chunk))
			return nil
		}
		if len(messages.Messages) == 0 {
			break
		}

		users := messages.MapUsers().UserToMap()

		for _, raw := range messages.Messages {
			msg, ok := raw.(*tg.Message)
			if !ok {
				continue
			}

			if time.Unix(int64(msg.Date), 0).Before(edge) {
				ub.logger.Info("history loaded", "chat_id", chat.ID, "saved", savedCount, "reason", "age limit")
				return nil
			}

			if !isValidMessage(msg) {
				continue
			}

			sender := extractSender(users, msg)
			if err := ub.buildAndSave(ctx, chat.ID, sender, msg); err != nil {
				ub.logger.Error("failed to save history message", "chat_id", chat.ID, "msg_id", msg.ID, "error", err)
				continue
			}

			savedCount++
			totalChars += utf8.RuneCountInString(strings.TrimSpace(msg.Message))
			if totalChars >= maxChars {
				ub.logger.Info("history loaded", "chat_id", chat.ID, "saved", savedCount, "reason", "char limit")
				return nil
			}
		}

		offsetID = messages.Messages[len(messages.Messages)-1].GetID()
	}

	ub.logger.Info("history loaded", "chat_id", chat.ID, "saved", savedCount, "reason", "reached chat start")
	return nil
}

func (ub *Userbot) buildAndSave(ctx context.Context, chatID int64, sender string, msg *tg.Message) error {
	m := model.Message{
		ID:     int64(msg.ID),
		Text:   msg.Message,
		SentAt: time.Unix(int64(msg.Date), 0),
		Sender: sender,
		ChatID: chatID,
	}

	if err := ub.storage.SaveMessage(ctx, m); err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	return nil
}

func extractSender(users map[int64]*tg.User, msg *tg.Message) string {
	if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
		if user, ok := users[fromUser.UserID]; ok {
			return user.FirstName + " " + user.LastName
		}
	}
	return "unknown"
}

func isValidMessage(msg *tg.Message) bool {
	cleanText := strings.TrimSpace(msg.Message)
	return utf8.RuneCountInString(cleanText) >= 10
}
