package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/ui"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage SSH profiles",
	Long:  "Create, switch, list, and manage SSH identity profiles for git accounts, remote servers, and client projects.",
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

		var (
			name     string
			profType string
			platform string
			email    string
			username string
			keyName  string
			keyType  string
		)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Profile name").
					Description("Short identifier, e.g. work or personal").
					Validate(huh.ValidateNotEmpty()).
					Value(&name),

				huh.NewSelect[string]().
					Title("Profile type").
					Options(
						huh.NewOption("git (GitHub / GitLab / Bitbucket)", config.ProfileTypeGit),
						huh.NewOption("server (SSH into remote machines)", config.ProfileTypeServer),
						huh.NewOption("client (custom / other)", config.ProfileTypeClient),
					).
					Value(&profType),
			),
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Platform").
					Options(
						huh.NewOption("GitHub", config.PlatformGitHub),
						huh.NewOption("GitLab", config.PlatformGitLab),
						huh.NewOption("Bitbucket", config.PlatformBitbucket),
						huh.NewOption("Other", config.PlatformOther),
					).
					Value(&platform),

				huh.NewInput().
					Title("Email").
					Validate(huh.ValidateNotEmpty()).
					Value(&email),

				huh.NewInput().
					Title("Username / git user").
					Validate(huh.ValidateNotEmpty()).
					Value(&username),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Key name").
					Description("Base filename for the key pair, e.g. work_ed25519").
					Validate(huh.ValidateNotEmpty()).
					Value(&keyName),

				huh.NewSelect[string]().
					Title("Key type").
					Options(
						huh.NewOption("ed25519 (recommended)", config.KeyTypeED25519),
						huh.NewOption("rsa 4096", config.KeyTypeRSA),
					).
					Value(&keyType),
			),
		)

		if err := form.Run(); err != nil {
			return fmt.Errorf("wizard cancelled: %w", err)
		}

		// Generate the key pair.
		comment := email
		keyInfo, err := app.keys.Generate(keyName, keyType, comment)
		if err != nil {
			return fmt.Errorf("generate key: %w", err)
		}

		// Create the profile record.
		p := &core.Profile{
			Name:     name,
			Type:     profType,
			Platform: platform,
			Email:    email,
			Username: username,
			KeyName:  keyName,
		}
		if err := app.profiles.Create(p); err != nil {
			return fmt.Errorf("create profile: %w", err)
		}

		// Sync the SSH config block.
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}

		pub, _ := app.keys.ReadPublicKey(keyInfo.Name)

		fmt.Printf("\nProfile %q created.\n\n", name)
		fmt.Printf("Public key (%s):\n%s\n", keyName, pub)
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
		return nil
	},
}

// ─── profile remove ──────────────────────────────────────────────────────────

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Delete a profile and optionally its key pair",
	Args:  cobra.ExactArgs(1),
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
			fmt.Scanln(&answer)
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

		src, err := app.profiles.Get(oldName)
		if err != nil {
			return err
		}
		// Create under new name, delete old.
		src.Name = newName
		if err := app.profiles.Create(src); err != nil {
			return err
		}
		if err := app.profiles.Delete(oldName); err != nil {
			return err
		}

		// Update any hosts that reference the old profile name.
		hosts, err := app.hosts.List()
		if err != nil {
			return err
		}
		for _, h := range hosts {
			if h.ProfileName == oldName {
				h.ProfileName = newName
				_ = app.hosts.Update(h)
			}
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

		// Pre-populate form with existing values.
		email := p.Email
		username := p.Username
		keyName := p.KeyName
		platform := p.Platform

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Email").Value(&email),
				huh.NewInput().Title("Username").Value(&username),
				huh.NewInput().Title("Key name").Value(&keyName),
				huh.NewInput().Title("Platform").Value(&platform),
			),
		)
		if err := form.Run(); err != nil {
			return fmt.Errorf("edit cancelled: %w", err)
		}

		p.Email = email
		p.Username = username
		p.KeyName = keyName
		p.Platform = platform

		if err := app.profiles.Update(p); err != nil {
			return err
		}
		if err := app.syncSSHConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ssh config sync: %v\n", err)
		}
		fmt.Printf("Updated profile %q.\n", p.Name)
		return nil
	},
}

func orDash(s string) string {
	if s == "" {
		return ui.StyleMuted.Render("—")
	}
	return s
}
