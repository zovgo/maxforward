package mf

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/google/uuid"
	"github.com/zovgo/maxforward/internal"
	"github.com/zovgo/maxproto"
	"go.uber.org/multierr"
)

func (conf Config) New() (*Forwarder, error) {
	if conf.Max.TokenEnvironment == "" {
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

	state internal.ValueWithMutex[struct {
		started bool
		closed  bool

		cl *maxproto.Client
	}]
	b *bot.Bot

	wg sync.WaitGroup
}

var ErrAlreadyStarted = errors.New("already started")

var ErrForwarderClosed = errors.New("forwarder closed")

func (f *Forwarder) Run(parent context.Context) error {
	select {
	case <-parent.Done():
		return parent.Err()
	default:
	}
	f.state.Lock()
	if f.state.V.closed {
		f.state.Unlock()
		return ErrForwarderClosed
	}
	if f.state.V.started {
		f.state.Unlock()
		return ErrAlreadyStarted
	}
	f.wg.Add(1)
	defer f.wg.Done()

	f.ctx, f.cancel = context.WithCancel(parent)
	f.state.V.started = true

	if err := f.createTelegramBot(); err != nil {
		f.cancel()
		f.state.Unlock()
		return fmt.Errorf("create telegram bot: %w", err)
	}
	f.wg.Go(f.dialLoop)
	f.conf.Logger.Info("starting telegram bot...")
	f.state.Unlock()
	f.b.Start(f.ctx)
	f.conf.Logger.Info("bot gracefully ended.")
	return nil
}

func (f *Forwarder) dialLoop() {
	var att int64
	for {
		f.state.Lock()
		select {
		case <-f.ctx.Done():
			f.state.Unlock()
			return
		default:
		}
		if f.state.V.closed {
			f.state.Unlock()
			return
		}
		att++
		l := f.conf.Logger.With("attempt", att)

		cl, err := f.dialMaxUnsafe(att)
		if err != nil {
			l.Error("dial to max", "err", err.Error())
			f.state.Unlock()
			return
		}
		l.Info("waiting for max messages...")
		f.state.Unlock()

		if err = cl.WaitForMessages(f.onMessage(cl)); err != nil {
			l.Error("wait for messages", "err", err.Error())
		}
		_ = cl.Close()
		if !f.conf.ReconnectOnClosure {
			l.Warn("reconnect on closure is disabled. closing forwarder...")
			_ = f.Close()
			return
		}
		f.state.Lock()
		if f.closedUnsafe() {
			return
		}
		// closedUnsafe releases mutex
		l.Info("reconnecting...")
	}
}

var ErrAlreadyClosed = errors.New("already closed")

func (f *Forwarder) Close() (multi error) {
	f.state.Lock()
	if f.state.V.closed {
		f.state.Unlock()
		return ErrAlreadyClosed
	}
	f.state.V.closed = true

	f.conf.Logger.Info("closing forwarder...")
	defer f.conf.Logger.Info("forwarder closed.")

	if !f.state.V.started {
		f.conf.Logger.Warn("closing while forwarder didn't start")
		f.state.Unlock()
		return
	}
	if err := f.closeTelegramBot(); err != nil {
		multi = multierr.Append(multi, fmt.Errorf("close telegram bot: %w", err))
	}
	func() {
		f.conf.Logger.Debug("closing max client...")
		if f.state.V.cl == nil {
			f.conf.Logger.Warn("closing while max client didn't start")
			return
		}
		if err := f.state.V.cl.Close(); err != nil && !errors.Is(err, maxproto.ErrAlreadyClosed) {
			multi = multierr.Append(multi, fmt.Errorf("close max client: %w", err))
			return
		}
		f.conf.Logger.Debug("closed max client.")
	}()
	f.conf.Logger.Debug("cancelling context...")
	f.cancel()
	f.state.Unlock()
	f.conf.Logger.Debug("waiting for wg end...")
	f.wg.Wait()
	f.conf.Logger.Debug("wait group ended.")
	return
}

func (f *Forwarder) closedUnsafe() bool {
	ok := f.state.V.closed
	if ok {
		f.state.Unlock()
		return true
	}
	select {
	case <-f.ctx.Done():
		f.state.Unlock()
		_ = f.Close()
		return false
	default:
	}
	f.state.Unlock()
	return false
}
