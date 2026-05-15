package mf

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/go-telegram/bot"
	"github.com/google/uuid"
	"github.com/zovgo/maxforward/internal"
	"github.com/zovgo/maxproto"
	"go.uber.org/multierr"
)

func (conf Config) New() (*Forwarder, error) {
	if conf.Max.TokenEnvironment == "" || conf.Max.GroupID == 0 {
		return nil, errors.New("invalid max config")
	}
	if conf.Telegram.TokenEnvironment == "" || conf.Telegram.GroupID == 0 {
		return nil, errors.New("invalid telegram config")
	}
	f := &Forwarder{}
	if conf.DeviceID != "" {
		f.device, _ = uuid.Parse(conf.DeviceID)
	}
	if f.device == uuid.Nil {
		f.device = uuid.New()
	}
	f.conf = conf
	return f, nil
}

type Forwarder struct {
	conf   Config
	device uuid.UUID

	ctx    context.Context
	cancel context.CancelFunc

	started internal.ValueWithMutex[bool]
	closed  atomic.Bool

	cl *maxproto.Client

	b *bot.Bot

	wg sync.WaitGroup
}

var ErrAlreadyStarted = errors.New("already started")

func (f *Forwarder) Run(parent context.Context) error {
	select {
	case <-parent.Done():
		return parent.Err()
	default:
	}
	f.started.Lock()
	if f.started.V {
		f.started.Unlock()
		return ErrAlreadyStarted
	}
	f.wg.Add(1)
	defer f.wg.Done()

	f.ctx, f.cancel = context.WithCancel(parent)
	defer f.cancel()

	f.started.V = true
	if err := f.createTelegramBot(); err != nil {
		f.started.Unlock()
		return fmt.Errorf("create telegram bot: %w", err)
	}
	if err := f.dialMax(); err != nil {
		f.started.Unlock()
		return fmt.Errorf("dial to max: %w", err)
	}
	f.conf.Logger.Info("starting telegram bot...")
	f.wg.Go(func() {
		f.conf.Logger.Info("waiting for max messages...")
		if err := f.cl.WaitForMessages(f.onMessage); err != nil && !errors.Is(err, maxproto.ErrClientClosed) && !errors.Is(err, context.Canceled) {
			f.conf.Logger.Error("wait for messages", "err", err.Error())
		}
		f.conf.Logger.Info("stopped waiting for max messages.")
	})
	f.started.Unlock()
	f.b.Start(f.ctx)
	f.conf.Logger.Info("bot gracefully ended.")
	return nil
}

var ErrAlreadyClosed = errors.New("already closed")

func (f *Forwarder) Close() (multi error) {
	if !f.closed.CompareAndSwap(false, true) {
		return ErrAlreadyClosed
	}
	f.conf.Logger.Info("closing forwarder...")
	defer f.conf.Logger.Info("forwarder closed.")

	f.started.Lock()
	defer f.started.Unlock()

	if !f.started.V {
		return
	}
	if err := f.closeTelegramBot(); err != nil {
		multi = multierr.Append(multi, fmt.Errorf("close telegram bot: %w", err))
	}
	if err := f.cl.Close(); err != nil {
		multi = multierr.Append(multi, fmt.Errorf("close max client: %w", err))
	}
	f.cancel()
	f.wg.Wait()
	return
}
