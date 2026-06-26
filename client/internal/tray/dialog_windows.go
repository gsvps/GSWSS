//go:build windows

package tray

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/gswss/gs-protocol/client/internal/config"
)

func showSettingsDialog(cfg config.Config) (config.Config, bool) {
	var (
		mw           *walk.MainWindow
		serverEdit   *walk.LineEdit
		passwordEdit *walk.LineEdit
		socksEdit    *walk.LineEdit
		httpEdit     *walk.LineEdit
		tlsCheck     *walk.CheckBox
		accepted     bool
		result       = cfg
	)

	if err := (MainWindow{
		AssignTo: &mw,
		Title:    "GSWSS 参数设置",
		MinSize:  Size{Width: 440, Height: 340},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}},
		Children: []Widget{
			Label{Text: "Worker 地址 (含 WebSocket 路径):"},
			LineEdit{AssignTo: &serverEdit, Text: cfg.Server},
			Label{Text: "密码 (PASSWORD):"},
			LineEdit{AssignTo: &passwordEdit, Text: cfg.Password, PasswordMode: true},
			Label{Text: "本地 SOCKS5:"},
			LineEdit{AssignTo: &socksEdit, Text: cfg.LocalSocks},
			Label{Text: "本地 HTTP 代理:"},
			LineEdit{AssignTo: &httpEdit, Text: cfg.LocalHTTP},
			CheckBox{AssignTo: &tlsCheck, Text: "启用 TLS (WSS)", Checked: cfg.TLS},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{
						Text: "连接测试",
						OnClicked: func() {
							testCfg := readForm(serverEdit, passwordEdit, socksEdit, httpEdit, tlsCheck)
							runTestDialog(testCfg)
						},
					},
					HSpacer{},
					PushButton{
						Text: "取消",
						OnClicked: func() {
							accepted = false
							mw.Close()
						},
					},
					PushButton{
						Text: "保存",
						OnClicked: func() {
							testCfg := readForm(serverEdit, passwordEdit, socksEdit, httpEdit, tlsCheck)
							if err := config.Validate(testCfg); err != nil {
								showError("参数无效", err.Error())
								return
							}
							result = testCfg
							accepted = true
							mw.Close()
						},
					},
				},
			},
		},
	}.Create()); err != nil {
		showError("打开设置", err.Error())
		return cfg, false
	}

	mw.Run()
	return result, accepted
}

func readForm(server, password, socks, http *walk.LineEdit, tls *walk.CheckBox) config.Config {
	cfg := config.Default()
	cfg.Server = server.Text()
	cfg.Password = password.Text()
	cfg.LocalSocks = socks.Text()
	cfg.LocalHTTP = http.Text()
	cfg.TLS = tls.Checked()
	cfg.LogLevel = "info"
	return cfg
}

func showInfo(title, msg string) {
	walk.MsgBox(nil, title, msg, walk.MsgBoxIconInformation)
}

func showError(title, msg string) {
	walk.MsgBox(nil, title, msg, walk.MsgBoxIconError)
}
