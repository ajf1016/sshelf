package commands

import "github.com/spf13/cobra"

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active profile and SSH identity at a glance",
	Long: `Print a quick summary of the currently active profile: git identity,
SSH key in use, agent status, and configured host aliases.`,
	RunE: notImplemented,
}
