//go:build windows

package tray

import (
	"context"
	"time"

	"github.com/gswss/gs-protocol/client/internal/config"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

func runTestDialog(cfg config.Config) {
	runOnUI(func() { runTestDialogUI(cfg) })
}

func runTestDialogUI(cfg config.Config) {
	if err := config.Validate(cfg); err != nil {
		showErrorUI("连接测试", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := transport.TestWorker(ctx, transport.RelayConfig{
		ServerURL: cfg.Server,
		Password:  cfg.Password,
		UseTLS:    cfg.TLS,
		Timeout:   15 * time.Second,
	})
	if err != nil {
		showErrorUI("连接测试失败", err.Error())
		return
	}
	showInfoUI("连接测试成功", "Worker 可达且认证通过")
}
