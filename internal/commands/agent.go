package commands

import "github.com/spf13/cobra"

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage the SSH agent",
	Long:  "Start, stop, and inspect the ssh-agent process and the keys it has loaded.",
}

func init() {
	agentCmd.AddCommand(
		agentStartCmd,
		agentStopCmd,
		agentStatusCmd,
		agentReloadCmd,
	)
}

var agentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start ssh-agent and load the active profile's keys",
	RunE:  notImplemented,
}

var agentStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running ssh-agent",
	RunE:  notImplemented,
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the agent PID and currently loaded keys",
	RunE:  notImplemented,
}

var agentReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload keys into the agent (run after a profile switch)",
	RunE:  notImplemented,
}
