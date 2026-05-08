package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/ui"
)

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Manage SSH host aliases",
	Long:  "Add, test, and connect to SSH host aliases managed in the sshelf block of ~/.ssh/config.",
}

func init() {
	// host add flags (alternative to interactive wizard)
	hostAddCmd.Flags().String("name", "", "host alias")
	hostAddCmd.Flags().String("hostname", "", "remote hostname or IP")
	hostAddCmd.Flags().String("user", "", "SSH user")
	hostAddCmd.Flags().Int("port", 0, "SSH port (default 22)")
	hostAddCmd.Flags().String("key", "", "identity key name")
	hostAddCmd.Flags().String("profile", "", "link to a profile")
	hostAddCmd.Flags().String("jump", "", "ProxyJump (bastion) host alias")

	hostRemoveCmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")

	hostListCmd.Flags().StringP("profile", "p", "", "filter by profile name")

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

// ─── host add ────────────────────────────────────────────────────────────────

var hostAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a host alias (interactive or --flags mode)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		hostname, _ := cmd.Flags().GetString("hostname")
		user, _ := cmd.Flags().GetString("user")
		port, _ := cmd.Flags().GetInt("port")
		keyName, _ := cmd.Flags().GetString("key")
		profileName, _ := cmd.Flags().GetString("profile")
		jumpHost, _ := cmd.Flags().GetString("jump")

		// If required flags are absent, run the interactive wizard.
		if name == "" || hostname == "" || user == "" {
			if err := runHostWizard(&name, &hostname, &user, &keyName, &profileName, &jumpHost); err != nil {
				return err
			}
		}

		h := &core.Host{
			Name:        name,
			Hostname:    hostname,
			User:        user,
			Port:        port,
			IdentityKey: keyName,
			ProfileName: profileName,
			JumpHost:    jumpHost,
		}
		if err := app.hosts.Create(h); err != nil {
			return err
		}
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		fmt.Printf("Added host alias %q → %s\n", name, hostname)
		return nil
	},
}

func runHostWizard(name, hostname, user, keyName, profileName, jumpHost *string) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Host alias").
				Description("Unique name for this host, e.g. github-work").
				Validate(huh.ValidateNotEmpty()).
				Value(name),

			huh.NewInput().
				Title("Hostname or IP").
				Validate(huh.ValidateNotEmpty()).
				Value(hostname),

			huh.NewInput().
				Title("SSH user").
				Validate(huh.ValidateNotEmpty()).
				Value(user),

			huh.NewInput().
				Title("Identity key name (optional)").
				Description("Leave blank to inherit from profile").
				Value(keyName),

			huh.NewInput().
				Title("Link to profile (optional)").
				Value(profileName),

			huh.NewInput().
				Title("ProxyJump / bastion alias (optional)").
				Value(jumpHost),
		),
	).Run()
}

// ─── host list ───────────────────────────────────────────────────────────────

var hostListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all host aliases grouped by profile",
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		filterProfile, _ := cmd.Flags().GetString("profile")

		var hosts []*core.Host
		if filterProfile != "" {
			hosts, err = app.hosts.ListByProfile(filterProfile)
		} else {
			hosts, err = app.hosts.List()
		}
		if err != nil {
			return err
		}
		if len(hosts) == 0 {
			fmt.Println("No host aliases. Run `sshelf host add` to create one.")
			return nil
		}

		// Group by profile.
		groups := make(map[string][]*core.Host)
		var order []string
		seen := make(map[string]bool)
		for _, h := range hosts {
			p := h.ProfileName
			if p == "" {
				p = "(no profile)"
			}
			if !seen[p] {
				order = append(order, p)
				seen[p] = true
			}
			groups[p] = append(groups[p], h)
		}

		for _, p := range order {
			fmt.Print(ui.Section("Profile: " + p))
			headers := []string{"ALIAS", "HOSTNAME", "USER", "PORT", "KEY", "JUMP"}
			rows := make([][]string, 0)
			for _, h := range groups[p] {
				portStr := ui.StyleMuted.Render("—")
				if h.Port != 0 {
					portStr = fmt.Sprintf("%d", h.Port)
				}
				jump := ui.StyleMuted.Render("—")
				if h.JumpHost != "" {
					jump = h.JumpHost
				}
				keyStr := ui.StyleMuted.Render("—")
				if h.IdentityKey != "" {
					keyStr = h.IdentityKey
				} else if h.ProfileName != "" {
					keyStr = ui.StyleMuted.Render("(from profile)")
				}
				rows = append(rows, []string{h.Name, h.Hostname, h.User, portStr, keyStr, jump})
			}
			fmt.Print(ui.Table(headers, rows))
			fmt.Println()
		}
		return nil
	},
}

// ─── host remove ─────────────────────────────────────────────────────────────

var hostRemoveCmd = &cobra.Command{
	Use:   "remove <alias>",
	Short: "Remove a host alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		alias := args[0]

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Printf("Remove host alias %q? [y/N] ", alias)
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := app.hosts.Delete(alias); err != nil {
			return err
		}
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		fmt.Printf("Removed host alias %q.\n", alias)
		return nil
	},
}

// ─── host test ───────────────────────────────────────────────────────────────

var hostTestCmd = &cobra.Command{
	Use:   "test <alias>",
	Short: "Test SSH connectivity to a host alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		alias := args[0]
		h, err := app.hosts.Get(alias)
		if err != nil {
			return err
		}

		fmt.Printf("Testing connection to %s (%s)...\n", alias, h.Hostname)

		sshArgs := []string{"-T", "-o", "StrictHostKeyChecking=accept-new",
			"-o", "ConnectTimeout=10"}

		// Resolve identity file.
		identityFile := resolveIdentityFile(app, h)
		if identityFile != "" {
			sshArgs = append(sshArgs, "-i", identityFile)
		}
		if h.Port != 0 {
			sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", h.Port))
		}
		if h.JumpHost != "" {
			sshArgs = append(sshArgs, "-J", h.JumpHost)
		}
		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", h.User, h.Hostname))

		// #nosec G204 — arguments are user-controlled SSH alias details, not arbitrary input
		cmd := exec.Command("ssh", sshArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		err = cmd.Run()
		if err != nil {
			// SSH -T returns exit 1 for git hosts like GitHub (expected).
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return nil
			}
			return fmt.Errorf("ssh test failed: %w", err)
		}
		return nil
	},
}

// ─── host connect ────────────────────────────────────────────────────────────

var hostConnectCmd = &cobra.Command{
	Use:   "connect <alias>",
	Short: "Open an interactive SSH session to a host alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		h, err := app.hosts.Get(args[0])
		if err != nil {
			return err
		}

		sshArgs := buildSSHArgs(app, h, nil)
		sshPath, err := exec.LookPath("ssh")
		if err != nil {
			return fmt.Errorf("ssh not found in PATH: %w", err)
		}

		// Replace the current process with SSH (exec-style).
		// #nosec G204 — arguments are from verified host configuration
		cmd := exec.Command(sshPath, sshArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
}

// ─── host copy-id ────────────────────────────────────────────────────────────

var hostCopyIDCmd = &cobra.Command{
	Use:   "copy-id <alias>",
	Short: "Push the active profile's public key to a remote host",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		h, err := app.hosts.Get(args[0])
		if err != nil {
			return err
		}

		// Determine the public key to push.
		identityFile := resolveIdentityFile(app, h)
		if identityFile == "" {
			return fmt.Errorf("no identity key associated with host %q", args[0])
		}
		pubKeyFile := identityFile + ".pub"

		sshCopyID, err := exec.LookPath("ssh-copy-id")
		if err != nil {
			return fmt.Errorf("ssh-copy-id not found in PATH: %w", err)
		}

		copyArgs := []string{"-i", pubKeyFile}
		if h.Port != 0 {
			copyArgs = append(copyArgs, "-p", fmt.Sprintf("%d", h.Port))
		}
		copyArgs = append(copyArgs, fmt.Sprintf("%s@%s", h.User, h.Hostname))

		// #nosec G204
		cmd := exec.Command(sshCopyID, copyArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
}

// ─── host jump ───────────────────────────────────────────────────────────────

var hostJumpCmd = &cobra.Command{
	Use:   "jump <alias>",
	Short: "Set or update the ProxyJump (bastion) host for an alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		h, err := app.hosts.Get(args[0])
		if err != nil {
			return err
		}
		via, _ := cmd.Flags().GetString("via")
		if via == "" {
			fmt.Printf("Current ProxyJump: %s\n", orDash(h.JumpHost))
			fmt.Print("New ProxyJump alias (leave blank to clear): ")
			fmt.Scanln(&via)
		}
		h.JumpHost = strings.TrimSpace(via)
		if err := app.hosts.Update(h); err != nil {
			return err
		}
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		if h.JumpHost == "" {
			fmt.Printf("Removed ProxyJump from %q.\n", args[0])
		} else {
			fmt.Printf("Set ProxyJump for %q → %s\n", args[0], h.JumpHost)
		}
		return nil
	},
}

func init() {
	hostJumpCmd.Flags().String("via", "", "bastion host alias to use as ProxyJump")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func resolveIdentityFile(app *appState, h *core.Host) string {
	keysDir := app.store.KeysDir()
	if h.IdentityKey != "" {
		return filepath.Join(keysDir, h.IdentityKey)
	}
	if h.ProfileName != "" {
		p, err := app.profiles.Get(h.ProfileName)
		if err == nil && p.KeyName != "" {
			return filepath.Join(keysDir, p.KeyName)
		}
	}
	return ""
}

func buildSSHArgs(app *appState, h *core.Host, extraArgs []string) []string {
	args := make([]string, 0, 8)
	identityFile := resolveIdentityFile(app, h)
	if identityFile != "" {
		args = append(args, "-i", identityFile)
	}
	if h.Port != 0 && h.Port != config.DefaultSSHPort {
		args = append(args, "-p", fmt.Sprintf("%d", h.Port))
	}
	if h.JumpHost != "" {
		args = append(args, "-J", h.JumpHost)
	}
	args = append(args, extraArgs...)
	args = append(args, fmt.Sprintf("%s@%s", h.User, h.Hostname))
	return args
}
