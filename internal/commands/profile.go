package commands

import "github.com/spf13/cobra"

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage SSH profiles",
	Long:  "Create, switch, list, and manage SSH identity profiles for git accounts, remote servers, and client projects.",
}

func init() {
	profileCmd.AddCommand(
		profileInitCmd,
		profileListCmd,
		profileSwitchCmd,
		profileRemoveCmd,
		profileShowCmd,
		profileCloneCmd,
		profileRenameCmd,
		profileEditCmd,
	)
}

var profileInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new SSH profile via interactive wizard",
	RunE:  notImplemented,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles with active indicator and key age",
	RunE:  notImplemented,
}

var profileSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Activate a profile and export git identity env vars",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a profile and optionally its keys",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print full details of a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var profileCloneCmd = &cobra.Command{
	Use:   "clone <name> <new-name>",
	Short: "Duplicate a profile as a starting point for a new one",
	Args:  cobra.ExactArgs(2),
	RunE:  notImplemented,
}

var profileRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a profile and update all references",
	Args:  cobra.ExactArgs(2),
	RunE:  notImplemented,
}

var profileEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Update profile fields interactively",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}
