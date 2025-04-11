package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"gopkg.in/telebot.v4"

	"classifier/pkg/lib"
)

type Bot struct {
	Bot         *telebot.Bot
	Subscribers Subscribers

	sid         string
	refreshRate time.Duration
	classify    bool
	crypto      *lib.Crypto
	key         string

	mu     sync.Mutex
	logger *log.Logger
}

type Subscribers = map[int64]*telebot.Chat

func New(token string, sid string, refreshRate time.Duration, classify bool, encryptionKey string, output io.Writer) (*Bot, error) {
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
		key:         encryptionKey,

		logger: logger,
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
	b.save()
	return nil
}

const savePath = "telegram.json"

func (b *Bot) save() {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := os.Create(savePath)
	if err != nil {
		b.logger.Error("Failed to create save file", "error", err, "path", savePath)
		return
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(b.Subscribers); err != nil {
		b.logger.Error("Failed to encode save file", "error", err, "path", savePath)
	}
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
	decoder := json.NewDecoder(f)
	subscribers := make(Subscribers)
	if err := decoder.Decode(&subscribers); err != nil {
		b.logger.Error("Failed to load save file", "error", err, "path", savePath)
		return
	}
	if subscribers != nil {
		b.Subscribers = subscribers
		b.logger.Debugf("Loaded %d subscribers from %s file", len(subscribers), savePath)
	}
}
