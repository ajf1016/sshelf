// Package platform provides OS-specific helpers.
package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// CopyToClipboard writes text to the system clipboard.
// It tries platform-native tools in order: pbcopy (macOS), then
// xclip / xsel / wl-copy on Linux. Returns an error when no tool is found.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		switch {
		case commandExists("xclip"):
			cmd = exec.Command("xclip", "-selection", "clipboard")
		case commandExists("xsel"):
			cmd = exec.Command("xsel", "--clipboard", "--input")
		case commandExists("wl-copy"):
			cmd = exec.Command("wl-copy")
		default:
			return fmt.Errorf("no clipboard utility found (install xclip, xsel, or wl-copy)")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy to clipboard: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
