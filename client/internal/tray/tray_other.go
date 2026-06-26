//go:build !windows

package tray

import "fmt"

// Run is only supported on Windows.
func Run(configPath string) error {
	return fmt.Errorf("tray mode is only supported on Windows")
}
