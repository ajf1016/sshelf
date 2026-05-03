package commands

import "github.com/spf13/cobra"

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Manage SSH host aliases",
	Long:  "Add, test, and connect to SSH host aliases managed in the sshelf block of ~/.ssh/config.",
}

func init() {
	hostCmd.AddCommand(
		hostAddCmd,
		hostListCmd,
		hostRemoveCmd,
		hostTestCmd,
		hostConnectCmd,
		hostCopyIDCmd,
		hostJumpCmd,
	)
}

var hostAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a host alias interactively",
	RunE:  notImplemented,
}

var hostListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all host aliases grouped by profile",
	RunE:  notImplemented,
}

var hostRemoveCmd = &cobra.Command{
	Use:   "remove <alias>",
	Short: "Remove a host alias",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var hostTestCmd = &cobra.Command{
	Use:   "test <alias>",
	Short: "Test SSH connectivity to a host alias",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var hostConnectCmd = &cobra.Command{
	Use:   "connect <alias>",
	Short: "Open an SSH session to a host alias",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var hostCopyIDCmd = &cobra.Command{
	Use:   "copy-id <alias>",
	Short: "Push the active profile's public key to a remote server",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var hostJumpCmd = &cobra.Command{
	Use:   "jump <alias>",
	Short: "Configure a ProxyJump (bastion) host for a host alias",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}
