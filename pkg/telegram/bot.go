package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"classifier/pkg/telegram/handlers"

	"github.com/charmbracelet/log"
)

type Bot struct {
	Telegram *handlers.Bot
}

type Bots interface {
	Registrar
	Starter
	Stopper
	Logger
}

type Registrar interface {
	Commands() error
	Handlers()
}

type Starter interface {
	Start() error
}

type Stopper interface {
	Stop() error
}

type Logger interface {
	Logger() *log.Logger
}

type Config struct {
	Token         string
	Output        io.Writer
	RefreshRate   string
	SID           string
	Classify      string
	EncryptionKey string
	Classes       string
	Context       context.Context
}

func New(config Config) (*Bot, error) {
	if config.Token == "" {
		return nil, errors.New("telegram token is required")
	}

	if config.SID == "" {
		return nil, errors.New("sid is required")
	}

	refreshRate := 30 * time.Second
	if config.RefreshRate != "" {
		if i, err := time.ParseDuration(config.RefreshRate); err == nil {
			refreshRate = i
		}
	}
	classes := []string{"cub"}
	if config.Classes != "" {
		classes = strings.Split(config.Classes, ",")
	}
	tgBot, err := handlers.New(
		config.Token,
		config.SID,
		refreshRate,
		config.Classify != "false",
		config.EncryptionKey,
		config.Output,
		config.Context,
		classes,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Telegram bot: %w", err)
	}

	return &Bot{Telegram: tgBot}, nil
}

func (b *Bot) Start() error {
	bot := b.Telegram
	bot.Logger().Debug(
		"Starting bot",
		"type", fmt.Sprintf("%T", bot),
	)
	err := bot.Start()
	if err != nil {
		bot.Logger().Error(
			"Failed to start bot",
			"type", fmt.Sprintf("%T", bot),
			"error", err,
		)
		return err
	}
	bot.Logger().Info(
		"Bot started successfully",
		"type", fmt.Sprintf("%T", bot),
	)

	bot.Logger().Debug(
		"Registering commands",
		"type", fmt.Sprintf("%T", bot),
	)
	err = bot.Commands()
	if err != nil {
		bot.Logger().Error(
			"Failed to register commands",
			"type", fmt.Sprintf("%T", bot),
			"error", err,
		)
		return err
	}
	bot.Logger().Debug(
		"Commands registered successfully",
		"type", fmt.Sprintf("%T", bot),
	)

	return bot.Handlers()
}

func (b *Bot) Wait() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop
	close(stop)
}

func (b *Bot) Shutdown() error {
	b.Telegram.Logger().Info("Shutting down bots")

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		err := b.Telegram.Stop()
		if err != nil {
			b.Telegram.Logger().Error(
				"Failed to stop bot",
				"type", fmt.Sprintf("%T", b.Telegram),
				"error", err,
			)
		} else {
			b.Telegram.Logger().Info(
				"Bot stopped successfully",
				"type", fmt.Sprintf("%T", b.Telegram),
			)
		}
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		b.Telegram.Logger().Error(
			"Bot did not stop in time",
			"type", fmt.Sprintf("%T", b.Telegram),
		)
	}

	return nil
}
