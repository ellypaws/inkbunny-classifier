package handlers

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
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

	sid         string
	refreshRate time.Duration
	classify    bool
	crypto      *lib.Crypto

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

func New(token string, sid string, refreshRate time.Duration, classify bool, encryptionKey string, output io.Writer, context context.Context) (*Bot, error) {
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
		classify:    classify,
		crypto:      crypto,

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
	References  map[string]*MessageRef `json:"references,omitempty"`
}

func (b *Bot) prune() {
	b.mu.Lock()
	defer b.mu.Unlock()
	maps.DeleteFunc(b.references, func(_ string, ref *MessageRef) bool {
		if ref == nil {
			return true
		}
		return len(ref.Messages) == 0
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
}
