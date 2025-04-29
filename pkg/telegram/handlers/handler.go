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
	falseButton      = telebot.Btn{Text: "False positive", Unique: "false"}
	undoButton       = telebot.Btn{Text: "Undo", Unique: "undo"}
	dangerButton     = telebot.Btn{Text: "⚠️ Danger", Unique: "danger"}
	undoDangerButton = telebot.Btn{Text: "Undo", Unique: "undo_danger"}
)

func (b *Bot) Handlers() error {
	b.Bot.Handle("/start", b.handleSubscribe)
	b.Bot.Handle("/stop", b.handleUnsubscribe)
	b.Bot.Handle(telebot.OnPhoto, b.handleUpload)
	b.Bot.Handle(&falseButton, b.handleReport(falsePositive))
	b.Bot.Handle(&undoButton, b.handleReport(undoFalsePositive))
	b.Bot.Handle(&dangerButton, b.handleReport(danger))
	b.Bot.Handle(&undoDangerButton, b.handleReport(undoDanger))
	return b.Watcher()
}

func (b *Bot) handleSubscribe(c telebot.Context) error {
	chat := c.Chat()
	if chat == nil {
		return errors.New("chat cannot be nil")
	}

	b.mu.RLock()
	_, ok := b.Blacklist[chat.ID]
	b.mu.RUnlock()
	if ok {
		b.logger.Warn("Blacklisted user is trying to subscribe", "id", chat.ID, "username", chat.Username)
		return nil
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
