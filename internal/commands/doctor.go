package commands

import "github.com/spf13/cobra"

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check SSH setup health and optionally auto-fix issues",
	Long: `Run a suite of checks against your SSH configuration, key permissions,
agent state, and git identity alignment.

Each check reports pass, warn, or fail. Use --fix to automatically
resolve all fixable issues (permissions, agent restart, missing keys).`,
	RunE: notImplemented,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "auto-fix all fixable issues")
	doctorCmd.Flags().String("check", "", "run a single named check (e.g. permissions, agent, git-identity)")
}
