package commands

import "github.com/spf13/cobra"

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage SSH key pairs",
	Long:  "Generate, import, rotate, copy, and back up SSH key pairs managed by sshelf.",
}

func init() {
	keyCmd.AddCommand(
		keyGenerateCmd,
		keyListCmd,
		keyAddCmd,
		keyRemoveCmd,
		keyRotateCmd,
		keyCopyCmd,
		keyBackupCmd,
		keyRestoreCmd,
	)
}

var keyGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new SSH key pair (ed25519 or RSA)",
	RunE:  notImplemented,
}

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed keys with age and linked profile",
	RunE:  notImplemented,
}

var keyAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Import an existing key into sshelf",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var keyRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a managed key (requires confirmation)",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var keyRotateCmd = &cobra.Command{
	Use:   "rotate <profile>",
	Short: "Generate a new key for a profile and archive the old one",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var keyCopyCmd = &cobra.Command{
	Use:   "copy <profile>",
	Short: "Copy a profile's public key to the clipboard",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var keyBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Encrypt and export managed keys to a vault file",
	RunE:  notImplemented,
}

var keyRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Decrypt and restore keys from a vault file",
	RunE:  notImplemented,
}
