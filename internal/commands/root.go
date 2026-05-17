package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
)

var (
	verbose   bool
	configDir string
	appInst   *appState
)

// appState holds initialized services shared across all command handlers.
// It is created lazily on first call to getApp(), so commands that don't
// touch the filesystem (e.g. completion) never create ~/.sshelf.
type appState struct {
	store     *core.Store
	profiles  *core.ProfileStore
	hosts     *core.HostStore
	keys      *core.KeyManager
	agent     *core.AgentManager
	sshConfig *core.SSHConfigWriter
	doctor    func() *core.DoctorRunner // constructed on demand
	verbose   bool
}

// syncSSHConfig is a convenience helper that loads current state and syncs the
// SSH config block. Commands that mutate hosts or profiles call this.
func (a *appState) syncSSHConfig() error {
	profiles, err := a.profiles.List()
	if err != nil {
		return fmt.Errorf("list profiles: %w", err)
	}
	hosts, err := a.hosts.List()
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	return a.sshConfig.Sync(profiles, hosts, a.store.KeysDir())
}

var rootCmd = &cobra.Command{
	Use:     config.AppName,
	Version: config.AppVersion,
	Short:   "Manage SSH profiles, keys, and identities from a single CLI",
	Long: `sshelf is a developer-focused CLI for managing SSH identities,
host aliases, key pairs, and agent sessions from a single machine.

Manage multiple git accounts, remote servers, and client environments
without ever hand-editing ~/.ssh/config again.`,
}

// Execute is the single entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "config directory (default: ~/.sshelf)")

	rootCmd.AddCommand(
		profileCmd,
		keyCmd,
		hostCmd,
		agentCmd,
		doctorCmd,
		whoamiCmd,
		importCmd,
		completionCmd,
		shellCmd,
	)
}

// getApp lazily initializes the application services on the first call and
// returns the shared instance. Subsequent calls return the cached instance.
func getApp() (*appState, error) {
	if appInst != nil {
		return appInst, nil
	}

	dir := configDir
	if dir == "" {
		var err error
		dir, err = config.DefaultConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolve config dir: %w", err)
		}
	}

	store, err := core.NewStore(dir)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	sshConfigPath, err := config.SSHConfigPath()
	if err != nil {
		return nil, fmt.Errorf("resolve ssh config path: %w", err)
	}

	profiles := core.NewProfileStore(store.ProfilesPath())
	hosts := core.NewHostStore(store.HostsPath())
	keys := core.NewKeyManager(store.KeysDir())
	agent := core.NewAgentManager(store.AgentEnvPath())
	sshCfg := core.NewSSHConfigWriter(sshConfigPath)

	appInst = &appState{
		store:     store,
		profiles:  profiles,
		hosts:     hosts,
		keys:      keys,
		agent:     agent,
		sshConfig: sshCfg,
		verbose:   verbose,
	}
	appInst.doctor = func() *core.DoctorRunner {
		return core.NewDoctorRunner(store, profiles, hosts, keys, agent)
	}
	return appInst, nil
}

