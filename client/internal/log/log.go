// Package log provides structured logging for the GS client.
package log

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

// Init initializes the global logger with the given level (stdout).
func Init(level string) error {
	return initLogger(level, []string{"stdout"}, []string{"stderr"})
}

// InitTray initializes logging to a file under the user config directory (for GUI tray mode).
func InitTray(level string) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	logDir := filepath.Join(dir, "gs-protocol")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "gs-client.log")
	return initLogger(level, []string{logPath}, []string{logPath})
}

func initLogger(level string, outputPaths, errorOutputPaths []string) error {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      outputPaths,
		ErrorOutputPaths: errorOutputPaths,
	}
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	l, err := cfg.Build()
	if err != nil {
		return err
	}
	logger = l
	return nil
}

// L returns the global logger, falling back to a no-op logger if uninitialized.
func L() *zap.Logger {
	if logger == nil {
		logger = zap.NewNop()
	}
	return logger
}

// Sync flushes any buffered log entries.
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}
