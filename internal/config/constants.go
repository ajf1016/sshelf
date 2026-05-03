package config

import (
	"os"
	"path/filepath"
)

const AppName = "sshelf"

// AppVersion is a var so the build pipeline can inject the git tag or commit
// SHA at link time: -ldflags="-X github.com/ajf1016/sshelf/internal/config.AppVersion=v1.2.3"
var AppVersion = "0.1.0"

const (

	// Data file names within the config directory.
	ProfilesFile = "profiles.toml"
	HostsFile    = "hosts.toml"
	AgentEnvFile = "agent.env"
	VaultFile    = "vault.enc"
	KeysDir      = "keys"

	// Delimiter comments written into ~/.ssh/config.
	SSHConfigBegin = "# BEGIN sshelf-managed — do not edit this block manually"
	SSHConfigEnd   = "# END sshelf-managed"

	// Supported key algorithms.
	KeyTypeED25519 = "ed25519"
	KeyTypeRSA     = "rsa"
	DefaultKeyType = KeyTypeED25519

	// Profile types.
	ProfileTypeGit    = "git"
	ProfileTypeServer = "server"
	ProfileTypeClient = "client"

	// Git hosting platforms.
	PlatformGitHub    = "github"
	PlatformGitLab    = "gitlab"
	PlatformBitbucket = "bitbucket"
	PlatformOther     = "other"

	// DefaultSSHPort is used when a host alias has no explicit port.
	DefaultSSHPort = 22
)

// POSIX permission bits as typed constants so callers don't use raw octal.
const (
	DirPerm        os.FileMode = 0700
	PrivateKeyPerm os.FileMode = 0600
	PublicKeyPerm  os.FileMode = 0644
	ConfigFilePerm os.FileMode = 0600
)

// DefaultConfigDir returns ~/.sshelf.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sshelf"), nil
}

// SSHConfigPath returns ~/.ssh/config.
func SSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}
