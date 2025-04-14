package handlers

import (
	"errors"

	"gopkg.in/telebot.v4"
)

// Deprecated: does not do anything
func (b *Bot) Commands() error {
	return nil
}

var (
	falseButton = telebot.Btn{Text: "Report as false positive", Unique: "false"}
	undoButton  = telebot.Btn{Text: "Undo", Unique: "undo"}
)

func (b *Bot) Handlers() error {
	b.Bot.Handle("/start", b.handleSubscribe)
	b.Bot.Handle("/stop", b.handleUnsubscribe)
	b.Bot.Handle(telebot.OnPhoto, b.handleUpload)
	b.Bot.Handle(&falseButton, b.handleReport(true))
	b.Bot.Handle(&undoButton, b.handleReport(false))
	return b.Watcher()
}

func (b *Bot) handleSubscribe(c telebot.Context) error {
	chat := c.Chat()
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	b.mu.Lock()
	b.Subscribers[chat.ID] = chat
	b.mu.Unlock()
	err := c.Send("Subscribed")
	if err != nil {
		return err
	}

	b.save()
	b.logger.Info("Subscribed successfully", "username", chat.Username, "id", chat.ID)

	return nil
}

func (b *Bot) handleUnsubscribe(c telebot.Context) error {
	chat := c.Chat()
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	b.mu.Lock()
	delete(b.Subscribers, chat.ID)
	b.mu.Unlock()
	err := c.Send("Unsubscribed")
	if err != nil {
		return err
	}

	b.save()
	b.logger.Info("Unsubscribed successfully", "username", chat.Username, "id", chat.ID)

	return nil
}
