package mf

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

type Config struct {
	LoadedFrom string       `json:"-"`
	Logger     *slog.Logger `json:"-"`

	Max       PlatformSettings `json:"max"`
	Telegram  PlatformSettings `json:"telegram"`
	DeviceID  string           `json:"device_id"`
	ChatCount int              `json:"chat_count"`

	ReconnectOnClosure bool `json:"reconnect_on_closure"`
}

type PlatformSettings struct {
	TokenEnvironment string `json:"token_environment"`
	GroupID          int64  `json:"group_id"`
}

const filePermission os.FileMode = 0777

func MustReadConfig(config *Config, path string) {
	err := ReadConfig(config, path)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		conf := DefaultConfig
		conf.LoadedFrom = path
		conf.MustWrite(path)
		*config = conf
		return
	}
	panic(err)
}

func ReadConfig(config *Config, path string) error {
	if config == nil {
		return errors.New("nil config")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file at %q: %w", path, err)
	}
	if err = json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("unmarshal JSON file data into config: %w", err)
	}
	config.LoadedFrom = path
	return nil
}

func (conf Config) MustWrite(path string) {
	if err := conf.Write(path); err != nil {
		panic(fmt.Errorf("server: config: MustWrite to %q: %w", path, err))
	}
}

func (conf Config) Write(path string) error {
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON config: %w", err)
	}
	if err = os.WriteFile(path, data, filePermission); err != nil {
		return fmt.Errorf("write config data to %q: %w", path, err)
	}
	return nil
}

var DefaultConfig = Config{ChatCount: 50}
