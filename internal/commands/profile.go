package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/ui"
	"github.com/ajf1016/sshelf/internal/wizard"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage SSH profiles",
	Long:  "Create, switch, list, and manage SSH identity profiles for git accounts, remote servers, and client projects.",
	Args:  cobra.ArbitraryArgs,
	RunE:  groupRunE,
}

func init() {
	profileRemoveCmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
	profileRemoveCmd.Flags().Bool("keep-key", false, "do not delete the associated key pair")

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

// ─── profile init ────────────────────────────────────────────────────────────

var profileInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new SSH profile via interactive wizard",
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		in, err := wizard.RunProfileInit()
		if err != nil {
			return err
		}

		keyInfo, err := app.keys.Generate(in.KeyName, in.KeyType, in.Email)
		if err != nil {
			return fmt.Errorf("generate key: %w", err)
		}

		p := &core.Profile{
			Name:     in.Name,
			Type:     in.Type,
			Platform: in.Platform,
			Email:    in.Email,
			Username: in.Username,
			KeyName:  in.KeyName,
		}
		if err := app.profiles.Create(p); err != nil {
			return fmt.Errorf("create profile: %w", err)
		}

		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}

		pub, _ := app.keys.ReadPublicKey(keyInfo.Name)

		fmt.Printf("\nProfile %q created.\n\n", in.Name)
		fmt.Printf("Public key (%s):\n%s\n", in.KeyName, pub)
		fmt.Println()
		fmt.Println(ui.StyleYellow.Render("Add the public key to your platform before using this profile."))
		return nil
	},
}

// ─── profile list ────────────────────────────────────────────────────────────

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles with active indicator and key age",
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		profiles, err := app.profiles.List()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Println("No profiles. Run `sshelf profile init` to create one.")
			return nil
		}

		headers := []string{"", "NAME", "TYPE", "PLATFORM", "EMAIL", "KEY", "AGE"}
		rows := make([][]string, 0, len(profiles))
		for _, p := range profiles {
			indicator := ui.Indicator(p.Active)
			age := ui.StyleMuted.Render("—")
			if kInfo, err := app.keys.Get(p.KeyName); err == nil {
				age = ui.FormatAge(kInfo.Age())
			}
			rows = append(rows, []string{
				indicator,
				p.Name,
				p.Type,
				orDash(p.Platform),
				p.Email,
				p.KeyName,
				age,
			})
		}
		fmt.Print(ui.Table(headers, rows))
		return nil
	},
}

// ─── profile show ────────────────────────────────────────────────────────────

var profileShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print full details of a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		p, err := app.profiles.Get(args[0])
		if err != nil {
			return err
		}

		label := ui.StyleHeader.Render
		val := func(s string) string { return orDash(s) }

		fmt.Printf("%s %s\n", label("Profile:"), p.Name)
		fmt.Printf("%s %s\n", label("Type:   "), val(p.Type))
		fmt.Printf("%s %s\n", label("Platform:"), val(p.Platform))
		fmt.Printf("%s %s\n", label("Email:  "), val(p.Email))
		fmt.Printf("%s %s\n", label("Username:"), val(p.Username))
		fmt.Printf("%s %s\n", label("Key:    "), val(p.KeyName))
		fmt.Printf("%s %v\n", label("Created:"), p.CreatedAt.Format("2006-01-02"))
		if p.Active {
			fmt.Printf("%s %s\n", label("Active: "), ui.StyleGreen.Render("yes"))
		}

		if p.KeyName != "" {
			if pub, err := app.keys.ReadPublicKey(p.KeyName); err == nil {
				fmt.Printf("\n%s\n%s\n", label("Public key:"), pub)
			}
		}
		return nil
	},
}

// ─── profile switch ──────────────────────────────────────────────────────────

var profileSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Activate a profile and print git identity env vars",
	Long: `Activate a profile and print shell export statements for the git identity.
Pipe into eval to apply in the current shell:
  eval $(sshelf profile switch work)`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		name := args[0]
		p, err := app.profiles.Get(name)
		if err != nil {
			return err
		}
		if err := app.profiles.SetActive(name); err != nil {
			return err
		}
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}

		// If agent is running, reload the profile key.
		if agentInfo, err := app.agent.Status(); err == nil && agentInfo.Running && p.KeyName != "" {
			keyPath := app.store.KeysDir() + "/" + p.KeyName
			_ = app.agent.LoadKey(keyPath, agentInfo.SocketPath)
		}

		// Print export lines for eval consumption.
		fmt.Printf("export GIT_AUTHOR_NAME=%q\n", p.Username)
		fmt.Printf("export GIT_COMMITTER_NAME=%q\n", p.Username)
		fmt.Printf("export GIT_AUTHOR_EMAIL=%q\n", p.Email)
		fmt.Printf("export GIT_COMMITTER_EMAIL=%q\n", p.Email)

		if agentInfo, err := app.agent.Status(); err == nil && agentInfo.Running {
			for _, line := range agentInfo.EnvLines() {
				fmt.Println(line)
			}
		}

		// If stdout is a terminal the user ran this command directly — the
		// exported vars above won't apply to their shell. Show a one-time hint.
		if stdoutIsTTY() {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  These env vars were printed but not applied to your shell.")
			fmt.Fprintln(os.Stderr, "  Run this once to fix that permanently:")
			fmt.Fprintln(os.Stderr, "    sshelf shell setup")
			fmt.Fprintln(os.Stderr, "  Then switch with:  sshelf-switch "+name)
		}
		return nil
	},
}

// ─── profile remove ──────────────────────────────────────────────────────────

var profileRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"delete"},
	Short:   "Delete a profile and optionally its key pair",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		name := args[0]
		p, err := app.profiles.Get(name)
		if err != nil {
			return err
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Printf("Delete profile %q", name)
			keepKey, _ := cmd.Flags().GetBool("keep-key")
			if !keepKey && p.KeyName != "" {
				fmt.Printf(" and key %q", p.KeyName)
			}
			fmt.Print("? [y/N] ")
			var answer string
			_, _ = fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := app.profiles.Delete(name); err != nil {
			return err
		}

		keepKey, _ := cmd.Flags().GetBool("keep-key")
		if !keepKey && p.KeyName != "" {
			_ = app.keys.Remove(p.KeyName)
		}

		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		fmt.Printf("Removed profile %q.\n", name)
		return nil
	},
}

// ─── profile clone ───────────────────────────────────────────────────────────

var profileCloneCmd = &cobra.Command{
	Use:   "clone <name> <new-name>",
	Short: "Duplicate a profile as a starting point for a new one",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		src, err := app.profiles.Get(args[0])
		if err != nil {
			return err
		}
		dst := &core.Profile{
			Name:     args[1],
			Type:     src.Type,
			Platform: src.Platform,
			Email:    src.Email,
			Username: src.Username,
			KeyName:  src.KeyName,
			Active:   false,
		}
		if err := app.profiles.Create(dst); err != nil {
			return err
		}
		fmt.Printf("Cloned profile %q → %q.\n", args[0], args[1])
		return nil
	},
}

// ─── profile rename ──────────────────────────────────────────────────────────

	var profileRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a profile and update all host references",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		oldName, newName := args[0], args[1]

		// Atomically rename in a single save cycle.
		if err := app.profiles.Rename(oldName, newName); err != nil {
			return err
		}

		// Update any hosts that reference the old profile name.
		hosts, err := app.hosts.List()
		if err != nil {
			return err
		}
		var hostErrs []string
		for _, h := range hosts {
			if h.ProfileName == oldName {
				h.ProfileName = newName
				if err := app.hosts.Update(h); err != nil {
					hostErrs = append(hostErrs, h.Name)
				}
			}
		}
		if len(hostErrs) > 0 {
			return fmt.Errorf("renamed profile but failed to update host references: %s", strings.Join(hostErrs, ", "))
		}

		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		fmt.Printf("Renamed profile %q → %q.\n", oldName, newName)
		return nil
	},
}

// ─── profile edit ────────────────────────────────────────────────────────────

var profileEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Update profile fields interactively",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		p, err := app.profiles.Get(args[0])
		if err != nil {
			return err
		}

		var keyNames []string
		if ks, _ := app.keys.List(); len(ks) > 0 {
			for _, k := range ks {
				keyNames = append(keyNames, k.Name)
			}
		}
		updated, err := wizard.RunProfileEdit(p, keyNames)
		if err != nil {
			return err
		}

		if err := app.profiles.Update(updated); err != nil {
			return err
		}
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		fmt.Printf("Updated profile %q.\n", updated.Name)
		return nil
	},
}

func orDash(s string) string {
	if s == "" {
		return ui.StyleMuted.Render("—")
	}
	return s
}
