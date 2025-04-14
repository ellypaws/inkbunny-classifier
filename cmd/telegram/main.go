package main

import (
	"os"
	"time"

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
	b, err := bot.New(bot.Config{
		Output:        os.Stdout,
		Token:         os.Getenv(EnvTelegramBotToken),
		RefreshRate:   os.Getenv(EnvTelegramRefreshRate),
		SID:           os.Getenv(EnvTelegramSID),
		Classify:      os.Getenv(EnvTelegramClassify),
		EncryptionKey: os.Getenv(EnvTelegramEncryptionKey),
	})
	if err != nil {
		log.Fatalf("error creating bot: %v", err)
	}

	err = b.Start()
	if err != nil {
		log.Fatalf("error starting bot: %v", err)
	}

	if err := b.Shutdown(); err != nil {
		log.Fatalf("error shutting down bot: %v", err)
	}

	time.Sleep(5 * time.Second)
}
