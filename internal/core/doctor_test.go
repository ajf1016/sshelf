package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajf1016/sshelf/internal/core"
)

func newTestDoctor(t *testing.T) (*core.DoctorRunner, *core.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := core.NewStore(dir)
	require.NoError(t, err)

	profiles := core.NewProfileStore(store.ProfilesPath())
	hosts := core.NewHostStore(store.HostsPath())
	keys := core.NewKeyManager(store.KeysDir())
	agent := core.NewAgentManager(store.AgentEnvPath())

	return core.NewDoctorRunner(store, profiles, hosts, keys, agent), store
}

func TestDoctorRunner_CheckConfigDir_Pass(t *testing.T) {
	runner, _ := newTestDoctor(t)
	results := runner.RunAll()
	configDirResult := findCheck(results, "config-dir")
	require.NotNil(t, configDirResult)
	assert.Equal(t, core.CheckPass, configDirResult.Status)
}

func TestDoctorRunner_CheckConfigDir_BadPermissions(t *testing.T) {
	runner, store := newTestDoctor(t)
	// Intentionally break the directory permissions.
	require.NoError(t, os.Chmod(store.Dir(), 0755))
	t.Cleanup(func() { _ = os.Chmod(store.Dir(), 0700) })

	result, err := runner.Run("config-dir")
	require.NoError(t, err)
	assert.Equal(t, core.CheckWarn, result.Status)
	assert.True(t, result.Fixable)

	// Fix should restore permissions.
	require.NoError(t, result.Fix())
	fi, _ := os.Stat(store.Dir())
	assert.Equal(t, os.FileMode(0700), fi.Mode().Perm())
}

func TestDoctorRunner_CheckKeyPermissions_NoKeys(t *testing.T) {
	runner, _ := newTestDoctor(t)
	result, err := runner.Run("permissions")
	require.NoError(t, err)
	assert.Equal(t, core.CheckPass, result.Status)
	assert.Contains(t, result.Message, "no managed keys")
}

func TestDoctorRunner_CheckKeyPermissions_BadPerm(t *testing.T) {
	runner, store := newTestDoctor(t)
	keys := core.NewKeyManager(store.KeysDir())

	info, err := keys.Generate("test_key", "ed25519", "")
	require.NoError(t, err)
	// Break permissions.
	require.NoError(t, os.Chmod(info.Path, 0644))

	result, err := runner.Run("permissions")
	require.NoError(t, err)
	assert.Equal(t, core.CheckFail, result.Status)
	assert.True(t, result.Fixable)

	// Fix should restore permissions.
	require.NoError(t, result.Fix())
	fi, _ := os.Stat(info.Path)
	assert.Equal(t, os.FileMode(0600), fi.Mode().Perm())
}

func TestDoctorRunner_CheckActiveProfile_NoProfile(t *testing.T) {
	runner, _ := newTestDoctor(t)
	result, err := runner.Run("active-profile")
	require.NoError(t, err)
	assert.Equal(t, core.CheckWarn, result.Status)
}

func TestDoctorRunner_CheckActiveProfile_WithActive(t *testing.T) {
	runner, store := newTestDoctor(t)
	profiles := core.NewProfileStore(store.ProfilesPath())
	p := newTestProfile("work")
	require.NoError(t, profiles.Create(p))
	require.NoError(t, profiles.SetActive("work"))

	result, err := runner.Run("active-profile")
	require.NoError(t, err)
	assert.Equal(t, core.CheckPass, result.Status)
}

func TestDoctorRunner_CheckKeyAge_Pass(t *testing.T) {
	runner, store := newTestDoctor(t)
	keys := core.NewKeyManager(store.KeysDir())
	_, err := keys.Generate("fresh_key", "ed25519", "")
	require.NoError(t, err)

	result, err := runner.Run("key-age")
	require.NoError(t, err)
	assert.Equal(t, core.CheckPass, result.Status)
}

func TestDoctorRunner_CheckOrphanedKeys_NoOrphans(t *testing.T) {
	runner, store := newTestDoctor(t)
	keys := core.NewKeyManager(store.KeysDir())
	profiles := core.NewProfileStore(store.ProfilesPath())

	_, err := keys.Generate("linked_key", "ed25519", "")
	require.NoError(t, err)

	p := newTestProfile("work")
	p.KeyName = "linked_key"
	require.NoError(t, profiles.Create(p))

	result, err := runner.Run("orphaned-keys")
	require.NoError(t, err)
	assert.Equal(t, core.CheckPass, result.Status)
}

func TestDoctorRunner_CheckOrphanedKeys_WithOrphan(t *testing.T) {
	runner, store := newTestDoctor(t)
	keys := core.NewKeyManager(store.KeysDir())
	_, err := keys.Generate("orphan_key", "ed25519", "")
	require.NoError(t, err)

	result, err := runner.Run("orphaned-keys")
	require.NoError(t, err)
	assert.Equal(t, core.CheckWarn, result.Status)
	assert.True(t, result.Fixable)

	// Fix archives the orphaned key.
	require.NoError(t, result.Fix())
	assert.False(t, fileExists(filepath.Join(store.KeysDir(), "orphan_key")))
	assert.True(t, fileExists(filepath.Join(store.KeysDir(), "orphan_key.bak")))
}

func TestDoctorRunner_RunAll_ReturnsAllChecks(t *testing.T) {
	runner, _ := newTestDoctor(t)
	results := runner.RunAll()
	assert.Len(t, results, 6)
}

func TestDoctorRunner_Run_UnknownCheck(t *testing.T) {
	runner, _ := newTestDoctor(t)
	_, err := runner.Run("does-not-exist")
	require.Error(t, err)
}

func TestDoctorRunner_AgentNotRunning(t *testing.T) {
	runner, _ := newTestDoctor(t)
	result, err := runner.Run("agent")
	require.NoError(t, err)
	// Without a real agent the check should warn.
	assert.Equal(t, core.CheckWarn, result.Status)
}

// findCheck returns the first result with the given name, or nil.
func findCheck(results []*core.CheckResult, name string) *core.CheckResult {
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
