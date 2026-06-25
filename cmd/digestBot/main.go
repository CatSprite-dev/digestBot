package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/CatSprite-dev/digestBot/internal/storage"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil {
		logger.Warn("no .env file, reading from environment")
	}

	apiIDStr := os.Getenv("TELEGRAM_API_ID")
	apiHash := os.Getenv("TELEGRAM_API_HASH")

	if apiIDStr == "" || apiHash == "" {
		logger.Error("missing required env vars", "vars", "TELEGRAM_API_ID, TELEGRAM_API_HASH")
		os.Exit(1)
	}

	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		logger.Error("TELEGRAM_API_ID must be a number", "error", err)
		os.Exit(1)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data.db"
	}

	db, err := storage.NewStorage(dbPath, logger)
	if err != nil {
		logger.Error("failed to open storage", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if err := db.Init(ctx); err != nil {
		logger.Error("failed to init storage", "error", err)
		os.Exit(1)
	}

	// Telegram клиент
	client := telegram.NewClient(apiID, apiHash, telegram.Options{})

	if err := client.Run(ctx, func(ctx context.Context) error {
		logger.Info("connected to Telegram")

		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("auth status: %w", err)
		}

		if !status.Authorized {
			phone := os.Getenv("TELEGRAM_PHONE")
			if phone == "" {
				return fmt.Errorf("TELEGRAM_PHONE is required for auth")
			}

			password := os.Getenv("TELEGRAM_2FA_PASSWORD")
			flow := auth.NewFlow(
				auth.Constant(phone, password, auth.CodeAuthenticatorFunc(func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
					fmt.Print("Enter code from Telegram: ")
					var code string
					_, err := fmt.Scan(&code)
					return code, err
				})),
				auth.SendCodeOptions{},
			)

			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}

		logger.Info("logged in", "name", self.FirstName+" "+self.LastName, "username", self.Username)

		api := tg.NewClient(client)
		_ = api
		return nil
	}); err != nil {
		logger.Error("telegram client error", "error", err)
		os.Exit(1)
	}
}
