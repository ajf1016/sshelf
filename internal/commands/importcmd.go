package commands

import "github.com/spf13/cobra"

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import an existing ~/.ssh/config into sshelf",
	Long: `Scan your existing SSH configuration for Host blocks and key files,
then interactively migrate them into sshelf management. No existing
data is deleted without explicit confirmation.`,
	RunE: notImplemented,
}
