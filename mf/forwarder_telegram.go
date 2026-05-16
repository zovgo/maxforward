package mf

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (f *Forwarder) createTelegramBot() error {
	f.conf.Logger.Info("creating telegram bot...")

	opts := []bot.Option{
		bot.WithDebugHandler(func(string, ...any) {}),
		bot.WithDefaultHandler(func(context.Context, *bot.Bot, *models.Update) {
		}),
	}
	b, err := bot.New(os.Getenv(f.conf.Telegram.TokenEnvironment), opts...) //why this doesn't support context
	if err != nil {
		return err
	}
	f.conf.Logger.Info("created telegram bot")
	f.b = b
	return nil
}

func (f *Forwarder) sendTelegramMessage(chat int64, msg string) error {
	f.conf.Logger.Info("sending telegram message", "chat", chat, "msg", fmt.Sprintln(msg))
	_, err := f.b.SendMessage(f.ctx, &bot.SendMessageParams{
		ChatID:              chat,
		Text:                msg,
		DisableNotification: true,
		ParseMode:           models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			PreferLargeMedia: new(true),
			ShowAboveText:    new(true),
		},
	})
	return err
}

func (f *Forwarder) closeTelegramBot() error {
	f.conf.Logger.Debug("closing telegram bot...")
	defer f.conf.Logger.Debug("closed telegram bot.")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if _, err := f.b.Close(ctx); err != nil {
		return err
	}
	return nil
}
