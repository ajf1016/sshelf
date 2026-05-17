package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// ImportCandidate is a host block or key file that can be imported.
type ImportCandidate struct {
	// For host blocks:
	Host *ParsedHost
	// For key files:
	KeyPath string
	KeyName string
}

// ImportScanner discovers existing SSH host blocks and key files that are not
// yet managed by sshelf.
type ImportScanner struct {
	sshDir string
	hosts  *HostStore
	keys   *KeyManager
	writer *SSHConfigWriter
}

// NewImportScanner returns a scanner for the given SSH directory.
func NewImportScanner(sshDir string, hosts *HostStore, keys *KeyManager, writer *SSHConfigWriter) *ImportScanner {
	return &ImportScanner{sshDir: sshDir, hosts: hosts, keys: keys, writer: writer}
}

// ScanHosts reads ~/.ssh/config and returns Host blocks not already tracked.
func (s *ImportScanner) ScanHosts() ([]*ParsedHost, error) {
	cfgPath := filepath.Join(s.sshDir, "config")
	if !utils.FileExists(cfgPath) {
		return nil, nil
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh config: %w", err)
	}
	// Skip the managed block and wildcard Host * entries.
	parsed := parseSSHConfig(string(data), true)

	// Filter out aliases that are already managed.
	var candidates []*ParsedHost
	for _, ph := range parsed {
		if ph.Alias == "" || ph.Alias == "*" {
			continue
		}
		if _, err := s.hosts.Get(ph.Alias); err == nil {
			continue // already managed
		}
		candidates = append(candidates, ph)
	}
	return candidates, nil
}

// ScanKeys returns key files in sshDir that are not yet in the managed keys directory.
func (s *ImportScanner) ScanKeys() ([]ImportCandidate, error) {
	entries, err := os.ReadDir(s.sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read ssh dir: %w", err)
	}

	managedKeys, _ := s.keys.List()
	managed := make(map[string]bool, len(managedKeys))
	for _, k := range managedKeys {
		managed[k.Name] = true
	}

	var candidates []ImportCandidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Identify private keys: skip .pub, known_hosts, config, authorized_keys.
		if strings.HasSuffix(name, ".pub") ||
			strings.HasSuffix(name, ".bak") ||
			name == "known_hosts" ||
			name == "known_hosts.old" ||
			name == "config" ||
			name == "authorized_keys" ||
			strings.HasPrefix(name, ".") {
			continue
		}
		if managed[name] {
			continue
		}
		candidates = append(candidates, ImportCandidate{
			KeyPath: filepath.Join(s.sshDir, name),
			KeyName: name,
		})
	}
	return candidates, nil
}

// ImportHost creates a Host record from a ParsedHost and updates the SSH config.
func (s *ImportScanner) ImportHost(ph *ParsedHost, profileName, keyName string, profiles []*Profile) error {
	port := 0
	if ph.Port != "" {
		_, _ = fmt.Sscanf(ph.Port, "%d", &port)
	}

	// Derive the key name from the IdentityFile path.
	if keyName == "" && ph.KeyFile != "" {
		keyName = filepath.Base(ph.KeyFile)
	}

	h := &Host{
		Name:        ph.Alias,
		Hostname:    ph.Hostname,
		User:        ph.User,
		Port:        port,
		IdentityKey: keyName,
		ProfileName: profileName,
	}

	if h.Hostname == "" {
		h.Hostname = ph.Alias
	}
	if h.User == "" {
		h.User = "root"
	}

	if err := s.hosts.Create(h); err != nil {
		return fmt.Errorf("create host %s: %w", ph.Alias, err)
	}

	// Re-sync the SSH config block.
	hosts, err := s.hosts.List()
	if err != nil {
		return err
	}
	return s.writer.Sync(profiles, hosts, s.keys.KeysDir())
}

// ImportKey copies a key file into the managed keys directory and fixes permissions.
func (s *ImportScanner) ImportKey(srcPath string) (*KeyInfo, error) {
	info, err := s.keys.Add(srcPath)
	if err != nil {
		return nil, err
	}
	// Enforce correct permissions on the imported key.
	if err := s.keys.FixPermissions(info.Name); err != nil {
		return nil, fmt.Errorf("fix permissions after import: %w", err)
	}
	return info, nil
}

// DefaultSSHDir returns the standard ~/.ssh path.
func DefaultSSHDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh"), nil
}

// EnsureSSHDir ensures ~/.ssh exists with the correct permissions.
func EnsureSSHDir() error {
	sshDir, err := DefaultSSHDir()
	if err != nil {
		return err
	}
	return utils.EnsureDir(sshDir, config.DirPerm)
}
