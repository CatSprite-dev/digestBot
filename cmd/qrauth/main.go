package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/joho/godotenv"
	"github.com/mdp/qrterminal/v3"
)

func main() {
	_ = godotenv.Load()

	apiID, _ := strconv.Atoi(os.Getenv("TELEGRAM_API_ID"))
	apiHash := os.Getenv("TELEGRAM_API_HASH")

	sessionPath := os.Getenv("SESSION_PATH")
	if sessionPath == "" {
		sessionPath = "./data/session.json"
	}

	dispatcher := tg.NewUpdateDispatcher()
	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		UpdateHandler:  dispatcher,
		SessionStorage: &session.FileStorage{Path: sessionPath},
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := client.Run(ctx, func(ctx context.Context) error {
		// уже авторизованы?
		if status, err := client.Auth().Status(ctx); err == nil && status.Authorized {
			fmt.Println("already authorized, session is valid")
			return nil
		}

		// канал-сигнал что QR отсканирован
		loggedIn := qrlogin.OnLoginToken(dispatcher)

		qr := client.QR()
		auth, err := qr.Auth(ctx, loggedIn, func(ctx context.Context, token qrlogin.Token) error {
			fmt.Println("\nScan this QR with Telegram on your phone:")
			fmt.Println("Settings → Devices → Link Desktop Device")
			qrterminal.GenerateHalfBlock(token.URL(), qrterminal.L, os.Stdout)
			return nil
		})

		// аккаунт с 2FA — после сканирования нужен облачный пароль
		if tgerr.Is(err, "SESSION_PASSWORD_NEEDED") {
			password := os.Getenv("TELEGRAM_2FA_PASSWORD")
			if password == "" {
				return fmt.Errorf("TELEGRAM_2FA_PASSWORD is required")
			}
			if _, err := client.Auth().Password(ctx, password); err != nil {
				return fmt.Errorf("password: %w", err)
			}
			fmt.Println("authorized with 2FA, session saved")
			return nil
		}
		if err != nil {
			return fmt.Errorf("qr auth: %w", err)
		}

		if u, ok := auth.User.AsNotEmpty(); ok {
			fmt.Printf("\nauthorized as %s (@%s)\n", u.FirstName, u.Username)
		}
		fmt.Println("session saved")
		return nil
	}); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
