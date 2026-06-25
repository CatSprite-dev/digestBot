package userbot

import (
	"context"
	"log/slog"

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
	ub.dispatcher.OnNewMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}
		ub.logger.Debug("new message received", "text", msg.Message)
		return nil
	})
}
