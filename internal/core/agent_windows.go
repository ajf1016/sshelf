//go:build windows

package core

import "os"

// isProcessAlive returns true if a process with pid can be found.
// On Windows, os.FindProcess always succeeds so this is a best-effort check.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.FindProcess(pid)
	return err == nil
}
