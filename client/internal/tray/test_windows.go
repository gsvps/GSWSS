//go:build windows

package tray

import (
	"context"
	"fmt"
	"time"

	"github.com/lxn/walk"

	"github.com/gswss/gs-protocol/client/internal/config"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

func runTestDialog(cfg config.Config) {
	runOnUI(func() { runTestDialogUI(nil, cfg) })
}

func runTestDialogUI(mw *walk.MainWindow, cfg config.Config) {
	if err := config.Validate(cfg); err != nil {
		showErrorUI("连接测试", err.Error())
		return
	}
	go runTestAsync(mw, cfg)
}

func runTestAsync(mw *walk.MainWindow, cfg config.Config) {
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("internal error: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err = transport.TestWorker(ctx, transport.RelayConfig{
			ServerURL: cfg.Server,
			Password:  cfg.Password,
			UseTLS:    cfg.TLS,
			Timeout:   15 * time.Second,
		})
	}()

	showTestResult(mw, err)
}

func showTestResult(mw *walk.MainWindow, err error) {
	show := func() {
		if err != nil {
			showErrorUI("连接测试失败", err.Error())
			return
		}
		showInfoUI("连接测试成功", "Worker 可达且认证通过\n\n请右键托盘「启动代理」后，浏览器设置 HTTP 代理 127.0.0.1:7890")
	}
	if mw != nil {
		mw.Synchronize(show)
		return
	}
	runOnUI(show)
}
