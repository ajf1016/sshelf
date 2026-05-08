//go:build !windows

package core

import (
	"os"
	"syscall"
)

// isProcessAlive returns true if a process with pid exists and is alive.
// Uses kill(pid, 0) on Unix — the null signal checks existence without delivery.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
