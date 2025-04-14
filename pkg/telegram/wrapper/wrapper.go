package wrapper

import (
	"gopkg.in/telebot.v4"
)

type edit interface {
	*telebot.ReplyMarkup | telebot.Location | inputtable | string
}

type inputtable interface {
	*telebot.Photo | *telebot.Audio | *telebot.Document | *telebot.Video | *telebot.Animation | *telebot.PaidInputtable
}

// using [telebot.Bot.extractOptions]
type options interface {
	*telebot.SendOptions | *telebot.ReplyMarkup | *telebot.ReplyParams | *telebot.Topic | telebot.Option | telebot.ParseMode | telebot.Entities
}

func Edit[w edit, o options](b *telebot.Bot, message *telebot.Message, what w, opts ...o) (*telebot.Message, error) {
	anyOpts := make([]any, len(opts))
	for i, opt := range opts {
		anyOpts[i] = opt
	}
	return b.Edit(message, what, anyOpts...)
}

type sendable interface {
	*telebot.Game | *telebot.LocationResult | *telebot.VenueResult |
		*telebot.Photo | *telebot.Audio | *telebot.Document |
		*telebot.Video | *telebot.Animation | *telebot.Voice |
		*telebot.VideoNote | *telebot.Sticker | *telebot.Location |
		*telebot.Venue | *telebot.Dice | *telebot.Invoice | *telebot.Poll | string
}

func Send[s sendable, o options](b *telebot.Bot, to telebot.Recipient, what s, opts ...o) (*telebot.Message, error) {
	anyOpts := make([]any, len(opts))
	for i, opt := range opts {
		anyOpts[i] = opt
	}
	return b.Send(to, what, anyOpts...)
}
