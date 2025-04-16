package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/charmbracelet/log"

	"classifier/pkg/telegram"
	"classifier/pkg/utils"
)

const (
	EnvTelegramBotToken      = "TELEGRAM_BOT_TOKEN"
	EnvTelegramRefreshRate   = "TELEGRAM_REFRESH_RATE"
	EnvTelegramSID           = "TELEGRAM_SID"
	EnvTelegramEncryptionKey = "TELEGRAM_ENCRYPT_KEY"
	EnvTelegramClassify      = "TELEGRAM_CLASSIFY"
)

func main() {
	defer utils.LogOutput(os.Stdout)()
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()
	b, err := bot.New(bot.Config{
		Output:        os.Stdout,
		Token:         os.Getenv(EnvTelegramBotToken),
		RefreshRate:   os.Getenv(EnvTelegramRefreshRate),
		SID:           os.Getenv(EnvTelegramSID),
		Classify:      os.Getenv(EnvTelegramClassify),
		EncryptionKey: os.Getenv(EnvTelegramEncryptionKey),
		Context:       ctx,
	})
	if err != nil {
		log.Fatalf("error creating bot: %v", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("error starting bot: %v", err)
	}

	if err := b.Shutdown(); err != nil {
		log.Fatalf("error shutting down bot: %v", err)
	}
}
