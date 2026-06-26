package userbot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

	m := model.Message{
		ID:     int64(msg.ID),
		Text:   msg.Message,
		SentAt: time.Unix(int64(msg.Date), 0),
		Sender: extractSender(entities, msg),
		ChatID: chatID,
	}

	if err := ub.storage.SaveMessage(ctx, m); err != nil {
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
			ID:       chat.ID,
			Username: chat.Username,
			Title:    chat.Title,
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

func extractSender(entities tg.Entities, msg *tg.Message) string {
	if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
		if user, ok := entities.Users[fromUser.UserID]; ok {
			return user.FirstName + " " + user.LastName
		}
	}
	return "unknown"
}
