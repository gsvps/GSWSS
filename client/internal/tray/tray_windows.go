//go:build windows

package tray

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/gswss/gs-protocol/client/internal/app"
	"github.com/gswss/gs-protocol/client/internal/config"
	"github.com/gswss/gs-protocol/client/internal/log"
	"github.com/gswss/gs-protocol/client/internal/transport"
)

// Run starts the Windows system tray application.
func Run(configPath string) error {
	if configPath == "" || configPath == "config.yaml" {
		configPath = config.DefaultConfigPath()
	}

	cfg, err := config.LoadOrDefault(configPath)
	if err != nil {
		return err
	}
	if err := log.Init(cfg.LogLevel); err != nil {
		return err
	}

	state := &trayState{
		configPath: configPath,
		cfg:        cfg,
		runner:     app.NewRunner(),
	}

	systray.Run(func() { state.onReady() }, func() { state.onExit() })
	return nil
}

type trayState struct {
	configPath string
	cfg        config.Config
	runner     *app.Runner
	mu         sync.Mutex

	mConnect *systray.MenuItem
	mStop    *systray.MenuItem
	mSettings *systray.MenuItem
	mTest    *systray.MenuItem
	mQuit    *systray.MenuItem
}

func (s *trayState) onReady() {
	if len(iconData) > 0 {
		systray.SetIcon(iconData)
	}
	systray.SetTitle("GSWSS")
	systray.SetTooltip("GS Protocol Client")

	s.mConnect = systray.AddMenuItem("启动代理", "Start SOCKS5/HTTP proxy")
	s.mStop = systray.AddMenuItem("停止代理", "Stop proxy")
	s.mSettings = systray.AddMenuItem("参数设置...", "Edit connection settings")
	s.mTest = systray.AddMenuItem("连接测试", "Test Worker connection")
	systray.AddSeparator()
	s.mQuit = systray.AddMenuItem("退出", "Quit GSWSS client")

	s.updateMenu()
	go s.eventLoop()
}

func (s *trayState) onExit() {
	s.runner.Stop()
	log.Sync()
}

func (s *trayState) eventLoop() {
	for {
		select {
		case <-s.mConnect.ClickedCh:
			s.handleConnect()
		case <-s.mStop.ClickedCh:
			s.handleStop()
		case <-s.mSettings.ClickedCh:
			s.handleSettings()
		case <-s.mTest.ClickedCh:
			s.handleTest()
		case <-s.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (s *trayState) handleConnect() {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()
	if err := config.Validate(cfg); err != nil {
		showInfo("启动失败", err.Error())
		return
	}
	if s.runner.Running() {
		showInfo("提示", "代理已在运行")
		return
	}
	if err := s.runner.Start(cfg); err != nil {
		showError("启动失败", err.Error())
		return
	}
	s.setTooltip("运行中")
	s.updateMenu()
}

func (s *trayState) handleStop() {
	s.runner.Stop()
	s.setTooltip("已停止")
	s.updateMenu()
}

func (s *trayState) handleSettings() {
	s.mu.Lock()
	current := s.cfg
	s.mu.Unlock()

	updated, ok := showSettingsDialog(current)
	if !ok {
		return
	}
	if err := config.Save(s.configPath, updated); err != nil {
		showError("保存失败", err.Error())
		return
	}
	s.mu.Lock()
	s.cfg = updated
	s.mu.Unlock()
	showInfo("已保存", fmt.Sprintf("配置已写入:\n%s", s.configPath))
}

func (s *trayState) handleTest() {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()
	if err := config.Validate(cfg); err != nil {
		showError("连接测试", err.Error())
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := transport.TestWorker(ctx, transport.RelayConfig{
			ServerURL: cfg.Server,
			Password:  cfg.Password,
			UseTLS:    cfg.TLS,
			Timeout:   15 * time.Second,
		})
		if err != nil {
			showError("连接测试失败", err.Error())
			return
		}
		showInfo("连接测试成功", fmt.Sprintf("Worker 可达且认证通过\n%s", cfg.Server))
	}()
}

func (s *trayState) updateMenu() {
	running := s.runner.Running()
	if running {
		s.mConnect.Disable()
		s.mStop.Enable()
		s.setTooltip("GSWSS — 运行中")
	} else {
		s.mConnect.Enable()
		s.mStop.Disable()
		s.setTooltip("GSWSS — 已停止")
	}
}

func (s *trayState) setTooltip(text string) {
	systray.SetTooltip(text)
}
