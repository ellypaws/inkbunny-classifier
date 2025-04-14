package handlers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/telebot.v4"

	"classifier/pkg/classify"
	"classifier/pkg/telegram/parser"
)

var warnNoPredictions = parser.Parse("**Could not determine**\n\n*All predictions are less than 75%*")

func (b *Bot) handleUpload(c telebot.Context) error {
	if err := c.Notify(randomActivity()); err != nil {
		return err
	}

	chat := c.Chat()
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	photo := c.Message().Photo
	if photo.FileID == "" {
		return errors.New("photo file id cannot be blank")
	}

	file, err := b.Bot.File(photo.MediaFile())
	if err != nil {
		return err
	}
	defer file.Close()

	folder := filepath.Join("telegram", chat.Username)
	fileName := filepath.Join(folder, photo.UniqueID)

	encrypt, err := b.crypto.Encrypt(file)
	if err != nil {
		b.logger.Errorf("Error downloading file %s: %v", photo.FileURL, err)
		return err
	}

	prediction, err := classify.DefaultCache.Predict(context.Background(), fileName, b.crypto.Key(), encrypt)
	if err != nil {
		b.logger.Error("Error classifying", "path", photo.FileURL, "err", err)
		return err
	}

	if len(prediction.Minimum(0.75)) == 0 {
		return c.Reply(warnNoPredictions, telebot.ModeMarkdownV2)
	}

	var sb strings.Builder
	for key, value := range prediction.Sorted() {
		fmt.Fprintf(&sb, "⚠️ **%s** = %.1f%%\n", key, value*100)
	}

	return c.Reply(strings.TrimRight(parser.Parse(sb.String()), "\n"), telebot.ModeMarkdownV2)
}
