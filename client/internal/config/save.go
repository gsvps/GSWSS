package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Server     string `yaml:"server"`
	Password   string `yaml:"password"`
	LocalSocks string `yaml:"local_socks"`
	LocalHTTP  string `yaml:"local_http"`
	TLS        bool   `yaml:"tls"`
	Heartbeat  int    `yaml:"heartbeat"`
	LogLevel   string `yaml:"log_level"`
}

// DefaultConfigPath returns the user-level config file path.
func DefaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "gs-protocol", "config.yaml")
}

// LoadOrDefault loads config from path or returns defaults if missing.
func LoadOrDefault(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Default(), nil
	}
	return Load(path)
}

// Save writes configuration to the given YAML file.
func Save(path string, cfg Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	heartbeat := int(cfg.Heartbeat / time.Second)
	if heartbeat <= 0 {
		heartbeat = 30
	}
	data, err := yaml.Marshal(&fileConfig{
		Server:     cfg.Server,
		Password:   cfg.Password,
		LocalSocks: cfg.LocalSocks,
		LocalHTTP:  cfg.LocalHTTP,
		TLS:        cfg.TLS,
		Heartbeat:  heartbeat,
		LogLevel:   cfg.LogLevel,
	})
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
