package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/zovgo/maxforward/internal/logger"
	"github.com/zovgo/maxforward/mf"
)

func main() {
	_ = godotenv.Load(".env")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conf := readConfig()
	conf.Logger = logger.NewPrettySlogger(os.Stdout, logger.Level(true))

	f, err := conf.New()
	if err != nil {
		panic(fmt.Errorf("new forwarder: %w", err))
	}
	defer f.Close() //nolint:errcheck

	err = f.Run(ctx)
	if err != nil {
		panic(fmt.Errorf("run forwarder: %w", err))
	}
}

func readConfig() (conf mf.Config) {
	mf.MustReadConfig(&conf, "config.json")
	return
}
