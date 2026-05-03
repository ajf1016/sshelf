package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// Store holds the resolved path to the sshelf config directory and exposes
// path helpers for all sub-resources. It is initialized once at startup via
// NewStore and shared across all command handlers.
type Store struct {
	dir string
}

// NewStore ensures the config directory and all required subdirectories exist
// with the correct permissions, then returns a ready-to-use Store. Safe to
// call on every startup — existing directories are not modified.
func NewStore(dir string) (*Store, error) {
	entries := []struct {
		path string
		perm os.FileMode
	}{
		{dir, config.DirPerm},
		{filepath.Join(dir, config.KeysDir), config.DirPerm},
	}

	for _, e := range entries {
		if err := utils.EnsureDir(e.path, e.perm); err != nil {
			return nil, fmt.Errorf("init store: %w", err)
		}
	}

	return &Store{dir: dir}, nil
}

func (s *Store) ProfilesPath() string { return filepath.Join(s.dir, config.ProfilesFile) }
func (s *Store) HostsPath() string    { return filepath.Join(s.dir, config.HostsFile) }
func (s *Store) KeysDir() string      { return filepath.Join(s.dir, config.KeysDir) }
func (s *Store) AgentEnvPath() string { return filepath.Join(s.dir, config.AgentEnvFile) }
func (s *Store) VaultPath() string    { return filepath.Join(s.dir, config.VaultFile) }
func (s *Store) Dir() string          { return s.dir }
