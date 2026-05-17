package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/platform"
	"github.com/ajf1016/sshelf/internal/ui"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage SSH key pairs",
	Long:  "Generate, import, rotate, copy, and back up SSH key pairs managed by sshelf.",
}

func init() {
	// generate flags
	keyGenerateCmd.Flags().StringP("type", "t", config.DefaultKeyType, "key type (ed25519 or rsa)")
	keyGenerateCmd.Flags().StringP("comment", "C", "", "key comment (defaults to <name>@sshelf)")
	keyGenerateCmd.Flags().StringP("name", "n", "", "key name (required)")
	_ = keyGenerateCmd.MarkFlagRequired("name")

	// rotate flags
	keyRotateCmd.Flags().StringP("type", "t", config.DefaultKeyType, "key type for the new pair (ed25519 or rsa)")

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
	Short: "Generate a new SSH key pair (ed25519 or rsa)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		keyType, _ := cmd.Flags().GetString("type")
		comment, _ := cmd.Flags().GetString("comment")
		if comment == "" {
			comment = name + "@sshelf"
		}

		info, err := app.keys.Generate(name, keyType, comment)
		if err != nil {
			return err
		}

		pub, err := app.keys.ReadPublicKey(info.Name)
		if err != nil {
			return err
		}

		fmt.Printf("Generated %s key: %s\n\n", info.Type, info.Path)
		fmt.Printf("Public key:\n%s\n", pub)
		return nil
	},
}

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed keys with age and linked profile",
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		keys, err := app.keys.List()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("No managed keys. Run `sshelf key generate --name <name>` to create one.")
			return nil
		}

		// Build a key → profile map.
		profiles, _ := app.profiles.List()
		keyProfile := make(map[string]string)
		for _, p := range profiles {
			keyProfile[p.KeyName] = p.Name
		}

		headers := []string{"NAME", "TYPE", "AGE", "PROFILE"}
		rows := make([][]string, 0, len(keys))
		for _, k := range keys {
			profile := ui.StyleMuted.Render("—")
			if p, ok := keyProfile[k.Name]; ok {
				profile = p
			}
			rows = append(rows, []string{
				k.Name,
				k.Type,
				ui.FormatAge(k.Age()),
				profile,
			})
		}
		fmt.Print(ui.Table(headers, rows))
		return nil
	},
}

var keyAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Import an existing key into sshelf",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		info, err := app.keys.Add(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Imported key: %s (%s)\n", info.Name, info.Type)
		return nil
	},
}

var keyRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a managed key (requires confirmation)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		name := args[0]
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Printf("Delete key %q and its public key? [y/N] ", name)
			var answer string
			_, _ = fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}
		if err := app.keys.Remove(name); err != nil {
			return err
		}
		fmt.Printf("Removed key: %s\n", name)
		return nil
	},
}

func init() {
	keyRemoveCmd.Flags().Bool("yes", false, "skip confirmation prompt")
}

var keyRotateCmd = &cobra.Command{
	Use:   "rotate <name>",
	Short: "Generate a new key pair and archive the old one",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		name := args[0]
		keyType, _ := cmd.Flags().GetString("type")

		info, err := app.keys.Rotate(name, keyType, name+"@sshelf")
		if err != nil {
			return err
		}

		pub, err := app.keys.ReadPublicKey(info.Name)
		if err != nil {
			return err
		}

		fmt.Printf("Rotated key %s (old backed up as %s.bak)\n\n", name, name)
		fmt.Printf("New public key:\n%s\n", pub)
		fmt.Println()
		fmt.Println(ui.StyleYellow.Render("Remember to add the new public key to GitHub/GitLab/remote authorized_keys."))

		// Re-sync SSH config with the new key in place.
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		return nil
	},
}

var keyCopyCmd = &cobra.Command{
	Use:   "copy <name>",
	Short: "Copy a key's public key to the clipboard",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		pub, err := app.keys.ReadPublicKey(args[0])
		if err != nil {
			return err
		}
		if err := platform.CopyToClipboard(pub); err != nil {
			// Fall back to printing if clipboard is unavailable.
			fmt.Println(pub)
			return fmt.Errorf("clipboard unavailable: %w", err)
		}
		fmt.Printf("Copied public key of %q to clipboard.\n", args[0])
		return nil
	},
}

var keyBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Encrypt and archive all managed keys to a vault file",
	Long: `Encrypt all key files in ~/.sshelf/keys/ into a single AES-256 encrypted
vault file. Keep this file somewhere safe (USB drive, cloud storage, etc.).

You will be prompted for a passphrase to protect the archive. You must
supply the same passphrase when restoring.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		outPath, _ := cmd.Flags().GetString("out")
		if outPath == "" {
			outPath = core.DefaultVaultPath(app.store.Dir())
		}

		// Prompt for passphrase with confirmation.
		var pass, confirm string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Passphrase").
					Description("Used to encrypt the backup — remember this, there is no recovery.").
					EchoMode(huh.EchoModePassword).
					Validate(func(s string) error {
						if len(s) < 8 {
							return fmt.Errorf("passphrase must be at least 8 characters")
						}
						return nil
					}).
					Value(&pass),
				huh.NewInput().
					Title("Confirm passphrase").
					EchoMode(huh.EchoModePassword).
					Value(&confirm),
			),
		)
		if err := form.Run(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		if pass != confirm {
			return fmt.Errorf("passphrases do not match")
		}

		fmt.Printf("Encrypting keys to %s ...\n", outPath)
		count, err := core.BackupKeys(app.store.KeysDir(), outPath, pass)
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		fmt.Printf("%s %d key file(s) backed up to: %s\n",
			ui.StyleGreen.Render("✓"), count, outPath)
		fmt.Println(ui.StyleYellow.Render("Store this file somewhere safe. Without the passphrase, it cannot be recovered."))
		return nil
	},
}

var keyRestoreCmd = &cobra.Command{
	Use:   "restore <vault-file>",
	Short: "Decrypt and restore keys from a vault file",
	Long: `Decrypt a vault file created by 'sshelf key backup' and restore the key
files into ~/.sshelf/keys/. Existing keys are skipped unless --force is passed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		vaultPath := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Prompt for passphrase.
		var pass string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Passphrase").
					Description("Passphrase used when the backup was created.").
					EchoMode(huh.EchoModePassword).
					Validate(huh.ValidateNotEmpty()).
					Value(&pass),
			),
		)
		if err := form.Run(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		count, err := core.RestoreKeys(vaultPath, app.store.KeysDir(), pass, force)
		if err != nil {
			return fmt.Errorf("restore: %w", err)
		}
		if count == 0 {
			fmt.Println("No new keys restored (all files already exist — use --force to overwrite).")
			return nil
		}
		fmt.Printf("%s %d key file(s) restored to: %s\n",
			ui.StyleGreen.Render("✓"), count, app.store.KeysDir())
		fmt.Println("Run `sshelf doctor` to verify permissions.")
		return nil
	},
}

func init() {
	keyBackupCmd.Flags().String("out", "", "output path for the vault file (default: ~/.sshelf/keys-backup-<date>.vault)")
	keyRestoreCmd.Flags().Bool("force", false, "overwrite existing key files")
}
