package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/ui"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage the SSH agent",
	Long:  "Start, stop, and inspect the ssh-agent process and the keys it has loaded.",
}

func init() {
	agentStartCmd.Flags().Bool("load-keys", true, "load all managed keys after starting")
	agentCmd.AddCommand(
		agentStartCmd,
		agentStopCmd,
		agentStatusCmd,
		agentReloadCmd,
	)
}

// ─── agent start ─────────────────────────────────────────────────────────────

var agentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start ssh-agent and load the active profile's keys",
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}

		loadKeys, _ := cmd.Flags().GetBool("load-keys")
		if loadKeys {
			info, err := app.agent.Reload(app.store.KeysDir())
			if err != nil {
				return err
			}
			printAgentStarted(info)
			return nil
		}

		info, err := app.agent.Start()
		if err != nil {
			return err
		}
		printAgentStarted(info)
		return nil
	},
}

// ─── agent stop ──────────────────────────────────────────────────────────────

var agentStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running ssh-agent",
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		if err := app.agent.Stop(); err != nil {
			return err
		}
		fmt.Println("ssh-agent stopped.")
		return nil
	},
}

// ─── agent status ────────────────────────────────────────────────────────────

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the agent PID, socket, and currently loaded keys",
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		info, err := app.agent.Status()
		if err != nil || !info.Running {
			fmt.Println(ui.StyleMuted.Render("ssh-agent is not running."))
			fmt.Println("Run `sshelf agent start` to start it.")
			return nil
		}

		label := ui.StyleHeader.Render
		fmt.Printf("%s %s\n", label("Agent PID:   "), fmt.Sprintf("%d", info.PID))
		fmt.Printf("%s %s\n", label("Socket:      "), info.SocketPath)
		fmt.Printf("%s %s\n", label("Status:      "), ui.StyleGreen.Render("running"))

		keys, err := app.agent.ListKeys(info.SocketPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: list agent keys: %v\n", err)
			return nil
		}
		if len(keys) == 0 {
			fmt.Printf("%s %s\n", label("Loaded keys: "), ui.StyleMuted.Render("none"))
		} else {
			fmt.Printf("%s\n", label("Loaded keys:"))
			for _, k := range keys {
				fmt.Printf("  %s\n", k)
			}
		}
		return nil
	},
}

// ─── agent reload ────────────────────────────────────────────────────────────

var agentReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Restart the agent and reload all managed keys",
	RunE: func(_ *cobra.Command, _ []string) error {
		app, err := getApp()
		if err != nil {
			return err
		}
		info, err := app.agent.Reload(app.store.KeysDir())
		if err != nil {
			return err
		}
		fmt.Println("Agent reloaded.")
		printAgentStarted(info)
		return nil
	},
}

func printAgentStarted(info interface{ EnvLines() []string }) {
	fmt.Println(ui.StyleGreen.Render("ssh-agent started."))
	fmt.Println("To apply in your shell:")
	for _, line := range info.EnvLines() {
		fmt.Println("  " + line)
	}
}
