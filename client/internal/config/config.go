// Package config loads and validates client configuration.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all client settings.
type Config struct {
	Server     string        `mapstructure:"server"`
	Password   string        `mapstructure:"password"`
	LocalSocks string        `mapstructure:"local_socks"`
	LocalHTTP  string        `mapstructure:"local_http"`
	TLS        bool          `mapstructure:"tls"`
	Mux        bool          `mapstructure:"mux"`
	Fetch      bool          `mapstructure:"fetch"`
	Heartbeat  time.Duration `mapstructure:"-"`
	LogLevel   string        `mapstructure:"log_level"`
}

type rawConfig struct {
	Server     string `mapstructure:"server"`
	Password   string `mapstructure:"password"`
	LocalSocks string `mapstructure:"local_socks"`
	LocalHTTP  string `mapstructure:"local_http"`
	TLS        bool   `mapstructure:"tls"`
	Mux        bool   `mapstructure:"mux"`
	Fetch      bool   `mapstructure:"fetch"`
	Heartbeat  int    `mapstructure:"heartbeat"`
	LogLevel   string `mapstructure:"log_level"`
}

// Default returns configuration with sensible defaults.
func Default() Config {
	return Config{
		LocalSocks: "127.0.0.1:1080",
		LocalHTTP:  "127.0.0.1:7890",
		TLS:        true,
		Fetch:      true,
		Heartbeat:  30 * time.Second,
		LogLevel:   "info",
	}
}

// Load reads configuration from the given file path.
func Load(path string) (Config, error) {
	cfg := Default()
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("GS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	var raw rawConfig
	if err := v.Unmarshal(&raw); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	cfg.Server = raw.Server
	cfg.Password = raw.Password
	cfg.LocalSocks = raw.LocalSocks
	cfg.LocalHTTP = raw.LocalHTTP
	cfg.TLS = raw.TLS
	cfg.Mux = raw.Mux
	if !v.IsSet("fetch") {
		cfg.Fetch = true
	} else {
		cfg.Fetch = raw.Fetch
	}
	cfg.LogLevel = raw.LogLevel
	if raw.Heartbeat > 0 {
		cfg.Heartbeat = time.Duration(raw.Heartbeat) * time.Second
	}
	return cfg, Validate(cfg)
}

// Validate checks required fields and value ranges.
func Validate(cfg Config) error {
	if cfg.Server == "" {
		return fmt.Errorf("config: server is required")
	}
	if cfg.Password == "" {
		return fmt.Errorf("config: password is required")
	}
	if cfg.LocalSocks == "" && cfg.LocalHTTP == "" {
		return fmt.Errorf("config: at least one of local_socks or local_http is required")
	}
	if cfg.Heartbeat <= 0 {
		return fmt.Errorf("config: heartbeat must be positive")
	}
	return nil
}
