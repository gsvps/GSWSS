//go:build !windows

package sysproxy

import "fmt"

// Enable is a no-op on non-Windows platforms.
func Enable(httpAddr string) error {
	return fmt.Errorf("system proxy is only supported on Windows")
}

// Disable is a no-op on non-Windows platforms.
func Disable() error {
	return nil
}
