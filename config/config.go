package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Router   RouterConfig   `yaml:"router"`
}

type TelegramConfig struct {
	Token          string  `yaml:"token"`
	AllowedUserIDs []int64 `yaml:"allowed_user_ids"`
}

type RouterConfig struct {
	XkeenPath string `yaml:"xkeen_path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Router.XkeenPath == "" {
		cfg.Router.XkeenPath = "/opt/sbin/xkeen"
	}
	return &cfg, nil
}
