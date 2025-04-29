package handlers

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"gopkg.in/telebot.v4"

	"classifier/pkg/lib"
	"classifier/pkg/utils"
)

type Bot struct {
	Bot         *telebot.Bot
	Subscribers Subscribers
	Blacklist   Subscribers

	sid         string
	refreshRate time.Duration
	threshold   float64
	classify    bool
	crypto      *lib.Crypto
	classes     []string

	references map[string]*MessageRef

	context context.Context
	mu      sync.RWMutex
	logger  *log.Logger
}

type MessageRef struct {
	Result   *Result             `json:"result,omitempty"`
	Messages []MessageWithButton `json:"messages,omitempty"`
	Reports  map[int64]bool      `json:"reports,omitempty"`
}

type MessageWithButton struct {
	Message *telebot.Message     `json:"message"`
	Button  *telebot.ReplyMarkup `json:"button"`
}

type Subscribers = map[int64]*telebot.Chat

func New(token string, sid string, refreshRate time.Duration, threshold float64, classify bool, encryptionKey string, output io.Writer, context context.Context, classes []string) (*Bot, error) {
	settings := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(settings)
	if err != nil {
		return nil, fmt.Errorf("error creating telegram bot: %w", err)
	}

	logger := log.NewWithOptions(output,
		log.Options{
			Level:           log.DebugLevel,
			ReportTimestamp: true,
			Prefix:          "[Telegram]",
		},
	)
	logger.SetColorProfile(termenv.TrueColor)

	crypto, err := lib.NewCrypto(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("error creating crypto: %w", err)
	}

	return &Bot{
		Bot:         bot,
		Subscribers: make(Subscribers),

		sid:         sid,
		refreshRate: refreshRate,
		threshold:   threshold,
		classify:    classify,
		crypto:      crypto,
		classes:     classes,

		references: make(map[string]*MessageRef),

		context: context,
		logger:  logger,
	}, nil
}

func (b *Bot) Logger() *log.Logger {
	return b.logger
}

func (b *Bot) Start() error {
	b.load()
	b.cleanup()
	go b.Bot.Start()
	b.logger.Info("Telegram bot started")
	return nil
}

func (b *Bot) Stop() error {
	b.logger.Info("Stopping Telegram bot")
	b.Bot.Stop()
	b.prune()
	b.save()
	return nil
}

const savePath = "telegram.json"

type Settings struct {
	Subscribers Subscribers            `json:"subscribers,omitempty"`
	Blacklist   Subscribers            `json:"blacklist,omitempty"`
	References  map[string]*MessageRef `json:"references,omitempty"`
}

func (b *Bot) prune() {
	b.mu.Lock()
	defer b.mu.Unlock()
	maps.DeleteFunc(b.references, func(_ string, ref *MessageRef) bool {
		if ref == nil {
			return true
		}
		if len(ref.Messages) == 0 {
			return true
		}
		for _, m := range ref.Messages {
			if m.Message != nil {
				return false
			}
		}
		return true
	})
}

func (b *Bot) save() {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := os.Create(savePath)
	if err != nil {
		b.logger.Error("Failed to create save file", "error", err, "path", savePath)
		return
	}
	defer f.Close()
	settings := Settings{
		Subscribers: b.Subscribers,
		Blacklist:   b.Blacklist,
		References:  b.references,
	}
	if err := utils.Encode(f, settings); err != nil {
		b.logger.Error("Failed to encode save file", "error", err, "path", savePath)
	}
	b.logger.Info("Saved telegram settings", "path", savePath, "subscribers", len(b.Subscribers), "references", len(b.references))
}

func (b *Bot) load() {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := os.Open(savePath)
	if err != nil {
		b.logger.Error("Failed to open save file", "error", err, "path", savePath)
		return
	}
	defer f.Close()
	settings, err := utils.Decode[Settings](f)
	if err != nil {
		b.logger.Error("Failed to load save file", "error", err, "path", savePath)
		return
	}
	if len(settings.Subscribers) > 0 {
		b.Subscribers = settings.Subscribers
		b.logger.Debugf("Loaded %d subscribers from %s file", len(settings.Subscribers), savePath)
	} else {
		b.logger.Warnf("No subscribers found in %s file", savePath)
	}
	if len(settings.References) > 0 {
		b.references = settings.References
		b.logger.Debugf("Loaded %d references from %s file", len(settings.References), savePath)
	} else {
		b.logger.Warnf("No references found in %s file", savePath)
	}
	if len(settings.Blacklist) > 0 {
		b.Blacklist = settings.Blacklist
		b.logger.Debugf("Loaded %d blacklisted users from %s file", len(settings.Blacklist), savePath)
		for id, user := range settings.Subscribers {
			if _, ok := b.Blacklist[id]; ok {
				b.logger.Warn("Blacklisted user was found in subscribers", "id", id, "username", user.Username)
				delete(settings.Subscribers, id)
			}
		}
	}
}

func (b *Bot) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, ref := range b.references {
		if ref == nil {
			b.logger.Warn("Reference is nil in cleanup", "submission", id)
			continue
		}
		ref.Messages = slices.DeleteFunc(ref.Messages, func(m MessageWithButton) bool {
			if m.Message == nil {
				b.logger.Warn("Message is nil in cleanup", "submission", id)
				return true
			}
			if m.Message.Chat == nil {
				b.logger.Warn("Message chat is nil in cleanup", "submission", id)
				return false
			}
			if _, ok := b.Blacklist[m.Message.Chat.ID]; !ok {
				return false
			}
			if err := b.Bot.Delete(m.Message); err != nil {
				b.logger.Error("Could not delete message from blacklisted user", "message", m.Message.ID, "id", m.Message.Chat.ID, "username", m.Message.Chat.Username, "error", err)
				edited, err := b.Bot.Edit(m.Message, "Detected filtered")
				if err != nil {
					b.logger.Warn("Could not redact message", "error", err)
					return false
				}
				*m.Message = *edited
				return true
			} else {
				b.logger.Warn("Deleted message from blacklisted user", "message", m.Message.ID, "id", m.Message.Chat.ID, "username", m.Message.Chat.Username)
				return true
			}
		})
	}
}
