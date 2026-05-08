package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/ui"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the active profile and SSH identity at a glance",
	Long:  `Print a quick summary of the currently active profile: git identity, SSH key in use, agent status, and configured host aliases.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		label := func(s string) string {
			return ui.StyleHeader.Render(fmt.Sprintf("%-14s", s))
		}

		// ── Active profile ──────────────────────────────────────
		p, err := app.profiles.ActiveProfile()
		if err != nil {
			fmt.Println(ui.StyleMuted.Render("No active profile. Run `sshelf profile switch <name>`."))
			return nil
		}

		fmt.Println(ui.StyleBlue.Bold(true).Render("Active Profile"))
		fmt.Println(ui.StyleMuted.Render("──────────────"))
		fmt.Printf("%s %s\n", label("Profile:"), p.Name)
		fmt.Printf("%s %s\n", label("Type:"), p.Type)
		if p.Platform != "" {
			fmt.Printf("%s %s\n", label("Platform:"), p.Platform)
		}
		fmt.Printf("%s %s\n", label("Email:"), p.Email)
		fmt.Printf("%s %s\n", label("Username:"), p.Username)
		fmt.Printf("%s %s\n", label("Key:"), p.KeyName)

		if p.KeyName != "" {
			if kInfo, err := app.keys.Get(p.KeyName); err == nil {
				age := ui.FormatAge(kInfo.Age())
				ageStr := age
				if kInfo.Age().Hours() > float64(365*24) {
					ageStr = ui.StyleYellow.Render(age + " (consider rotating)")
				}
				fmt.Printf("%s %s\n", label("Key age:"), ageStr)
			}
		}

		// ── Agent ───────────────────────────────────────────────
		fmt.Println()
		fmt.Println(ui.StyleBlue.Bold(true).Render("Agent"))
		fmt.Println(ui.StyleMuted.Render("─────"))
		agentInfo, agentErr := app.agent.Status()
		if agentErr != nil || !agentInfo.Running {
			fmt.Printf("%s %s\n", label("Status:"), ui.StyleRed.Render("not running"))
			fmt.Printf("  Run `sshelf agent start` to load your keys.\n")
		} else {
			fmt.Printf("%s %s (PID %d)\n", label("Status:"), ui.StyleGreen.Render("running"), agentInfo.PID)
			if keys, err := app.agent.ListKeys(agentInfo.SocketPath); err == nil {
				fmt.Printf("%s %d\n", label("Loaded keys:"), len(keys))
			}
		}

		// ── Host aliases ─────────────────────────────────────────
		hosts, err := app.hosts.ListByProfile(p.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: list hosts: %v\n", err)
			return nil
		}
		if len(hosts) > 0 {
			fmt.Println()
			fmt.Println(ui.StyleBlue.Bold(true).Render("Host Aliases"))
			fmt.Println(ui.StyleMuted.Render("────────────"))
			for _, h := range hosts {
				fmt.Printf("  %-20s → %s\n", h.Name, h.Hostname)
			}
		}
		return nil
	},
}
