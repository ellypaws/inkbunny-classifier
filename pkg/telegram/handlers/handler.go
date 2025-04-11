package handlers

import (
	"errors"
	"fmt"

	"gopkg.in/telebot.v4"

	"classifier/pkg/telegram/parser"
)

// Deprecated: does not do anything
func (b *Bot) Commands() error {
	return nil
}

func (b *Bot) Handlers() error {
	b.Bot.Handle("/start", b.handleSubscribe)
	b.Bot.Handle("/stop", b.handleUnsubscribe)
	return b.Watcher()
}

func (b *Bot) Notify(content string) ([]*telebot.Message, error) {
	if len(b.Subscribers) == 0 {
		b.logger.Warn("Cannot send message - no subscribers")
		return nil, nil
	}

	message := parser.Parse(content)
	var reference []*telebot.Message
	for id, recipient := range b.Subscribers {
		if recipient == nil {
			b.logger.Warnf("%d has no recipient", id)
			continue
		}
		b.logger.Debug("Sending message to Telegram", "user_id", id)
		message, err := b.Bot.Send(recipient, message, &telebot.SendOptions{ParseMode: telebot.ModeMarkdownV2})
		if err != nil {
			b.logger.Error("Failed to send message", "error", err, "user_id", id)
			return nil, fmt.Errorf("error sending to telegram: %w", err)
		}

		b.logger.Info("Message sent successfully", "user_id", id)
		reference = append(reference, message)
	}

	return reference, nil
}

func (b *Bot) Edit(reference *telebot.Message, content any) (*telebot.Message, error) {
	if id, chatID := reference.MessageSig(); id == "" || chatID == 0 {
		b.logger.Warn("Cannot edit message - invalid reference")
		return nil, fmt.Errorf("invalid reference")
	}

	b.logger.Debug(
		"Editing message in Telegram",
		"message_id", reference.ID,
		"channel_id", reference.Chat.ID,
		"thread_id", reference.ThreadID,
	)

	edited, err := b.Bot.Edit(reference, content, &telebot.SendOptions{
		ParseMode: telebot.ModeMarkdownV2,
		ThreadID:  reference.ThreadID,
	})
	if err != nil {
		b.logger.Error(
			"Failed to edit message in Telegram",
			"error", err,
			"message_id", reference.ID,
			"chat_id", reference.Chat.ID,
			"thread_id", reference.ThreadID,
		)
		return nil, fmt.Errorf("error editing message: %w", err)
	}

	b.logger.Info(
		"Successfully edited message in Telegram",
		"message_id", reference.ID,
		"chat_id", reference.Chat.ID,
		"thread_id", reference.ThreadID,
	)

	return edited, nil
}

func (b *Bot) Delete(reference *telebot.Message) error {
	if id, chatID := reference.MessageSig(); id == "" || chatID == 0 {
		b.logger.Warn("Cannot delete message - invalid reference")
		return fmt.Errorf("invalid reference")
	}

	b.logger.Debug(
		"Deleting message from Telegram",
		"message_id", reference.ID,
		"chat_id", reference.Chat.ID,
		"thread_id", reference.ThreadID,
	)

	err := b.Bot.Delete(reference)
	if err != nil {
		b.logger.Error(
			"Failed to delete message from Telegram",
			"error", err,
			"message_id", reference.ID,
			"chat_id", reference.Chat.ID,
			"thread_id", reference.ThreadID,
		)
		return fmt.Errorf("error deleting message: %w", err)
	}

	b.logger.Info(
		"Successfully deleted message from Telegram",
		"message_id", reference.ID,
		"chat_id", reference.Chat.ID,
		"thread_id", reference.ThreadID,
	)

	return nil
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

	return nil
}
