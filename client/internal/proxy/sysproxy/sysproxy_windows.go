//go:build windows

package sysproxy

import (
	"fmt"
	"sync"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const settingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

var (
	wininet           = syscall.NewLazyDLL("wininet.dll")
	internetSetOption = wininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

// Snapshot stores previous Windows proxy settings for restore.
type Snapshot struct {
	Enable   uint32
	Server   string
	Override string
}

var (
	mu       sync.Mutex
	snapshot *Snapshot
)

// Enable sets the system HTTP proxy and saves the previous settings once.
func Enable(httpAddr string) error {
	if httpAddr == "" {
		return fmt.Errorf("proxy address is required")
	}
	mu.Lock()
	defer mu.Unlock()

	if snapshot == nil {
		prev, err := read()
		if err != nil {
			return err
		}
		snapshot = &prev
	}

	if err := write(1, httpAddr, "<local>"); err != nil {
		return err
	}
	refresh()
	return nil
}

// Disable restores the proxy settings saved by Enable.
func Disable() error {
	mu.Lock()
	defer mu.Unlock()
	if snapshot == nil {
		return nil
	}
	if err := write(snapshot.Enable, snapshot.Server, snapshot.Override); err != nil {
		return err
	}
	refresh()
	snapshot = nil
	return nil
}

func read() (Snapshot, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, settingsKey, registry.QUERY_VALUE)
	if err != nil {
		return Snapshot{}, err
	}
	defer k.Close()

	var snap Snapshot
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		snap.Enable = uint32(v)
	}
	if v, _, err := k.GetStringValue("ProxyServer"); err == nil {
		snap.Server = v
	}
	if v, _, err := k.GetStringValue("ProxyOverride"); err == nil {
		snap.Override = v
	}
	return snap, nil
}

func write(enable uint32, server, override string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, settingsKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", enable); err != nil {
		return err
	}
	if enable == 0 {
		return nil
	}
	if err := k.SetStringValue("ProxyServer", server); err != nil {
		return err
	}
	return k.SetStringValue("ProxyOverride", override)
}

func refresh() {
	internetSetOption.Call(0, internetOptionSettingsChanged, 0, 0)
	internetSetOption.Call(0, internetOptionRefresh, 0, 0)
}
