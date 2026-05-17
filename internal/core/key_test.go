package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/utils"
)

func newKeyManager(t *testing.T) *core.KeyManager {
	t.Helper()
	dir := t.TempDir()
	return core.NewKeyManager(dir)
}

func TestKeyManager_GenerateAndGet(t *testing.T) {
	mgr := newKeyManager(t)

	info, err := mgr.Generate("test_ed25519", config.KeyTypeED25519, "test@example.com")
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "test_ed25519", info.Name)
	assert.Equal(t, config.KeyTypeED25519, info.Type)
	assert.Equal(t, "test@example.com", info.Comment)

	// Private key must exist and be 0600.
	fi, err := os.Stat(info.Path)
	require.NoError(t, err)
	assert.Equal(t, config.PrivateKeyPerm, fi.Mode().Perm())

	// Public key must exist and be 0644.
	pubFi, err := os.Stat(info.PublicKeyPath())
	require.NoError(t, err)
	assert.Equal(t, config.PublicKeyPerm, pubFi.Mode().Perm())
}

func TestKeyManager_GenerateDuplicate(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Generate("my_key", config.KeyTypeED25519, "")
	require.NoError(t, err)

	_, err = mgr.Generate("my_key", config.KeyTypeED25519, "")
	var target *utils.ErrAlreadyExists
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "key", target.Resource)
}

func TestKeyManager_GenerateInvalidType(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Generate("bad", "dsa", "")
	var target *utils.ErrInvalidInput
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "key_type", target.Field)
}

func TestKeyManager_GenerateEmptyName(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Generate("", config.KeyTypeED25519, "")
	var target *utils.ErrInvalidInput
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "name", target.Field)
}

func TestKeyManager_ListSortedAlphabetically(t *testing.T) {
	mgr := newKeyManager(t)
	for _, name := range []string{"zzz_key", "aaa_key", "mmm_key"} {
		_, err := mgr.Generate(name, config.KeyTypeED25519, "")
		require.NoError(t, err)
	}

	keys, err := mgr.List()
	require.NoError(t, err)
	require.Len(t, keys, 3)
	assert.Equal(t, "aaa_key", keys[0].Name)
	assert.Equal(t, "mmm_key", keys[1].Name)
	assert.Equal(t, "zzz_key", keys[2].Name)
}

func TestKeyManager_ListEmpty(t *testing.T) {
	mgr := newKeyManager(t)
	keys, err := mgr.List()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestKeyManager_GetNotFound(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Get("ghost")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestKeyManager_Remove(t *testing.T) {
	mgr := newKeyManager(t)
	info, err := mgr.Generate("del_key", config.KeyTypeED25519, "")
	require.NoError(t, err)

	require.NoError(t, mgr.Remove(info.Name))

	assert.False(t, utils.FileExists(info.Path))
	assert.False(t, utils.FileExists(info.PublicKeyPath()))
}

func TestKeyManager_Remove_NotFound(t *testing.T) {
	mgr := newKeyManager(t)
	err := mgr.Remove("ghost")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestKeyManager_ReadPublicKey(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Generate("pub_key", config.KeyTypeED25519, "mycomment")
	require.NoError(t, err)

	pub, err := mgr.ReadPublicKey("pub_key")
	require.NoError(t, err)
	assert.Contains(t, pub, "ssh-ed25519")
	assert.Contains(t, pub, "mycomment")
}

func TestKeyManager_ReadPublicKey_NotFound(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.ReadPublicKey("ghost")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestKeyManager_FixPermissions(t *testing.T) {
	mgr := newKeyManager(t)
	info, err := mgr.Generate("fix_key", config.KeyTypeED25519, "")
	require.NoError(t, err)

	// Intentionally break permissions.
	require.NoError(t, os.Chmod(info.Path, 0644))
	require.NoError(t, os.Chmod(info.PublicKeyPath(), 0600))

	require.NoError(t, mgr.FixPermissions(info.Name))

	fi, _ := os.Stat(info.Path)
	assert.Equal(t, config.PrivateKeyPerm, fi.Mode().Perm())
	pubFi, _ := os.Stat(info.PublicKeyPath())
	assert.Equal(t, config.PublicKeyPerm, pubFi.Mode().Perm())
}

func TestKeyManager_CheckPermissions_BadPriv(t *testing.T) {
	mgr := newKeyManager(t)
	info, err := mgr.Generate("check_key", config.KeyTypeED25519, "")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(info.Path, 0644))

	err = mgr.CheckPermissions(info.Name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check_key")
}

func TestKeyManager_Add(t *testing.T) {
	// Generate a key in one directory and import into another.
	srcMgr := newKeyManager(t)
	srcInfo, err := srcMgr.Generate("import_key", config.KeyTypeED25519, "test")
	require.NoError(t, err)

	dstMgr := newKeyManager(t)
	imported, err := dstMgr.Add(srcInfo.Path)
	require.NoError(t, err)
	assert.Equal(t, "import_key", imported.Name)
	assert.True(t, utils.FileExists(imported.Path))
}

func TestKeyManager_Add_NotFound(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Add("/nonexistent/path/key")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestKeyManager_Add_Duplicate(t *testing.T) {
	mgr := newKeyManager(t)
	info, _ := mgr.Generate("dup_key", config.KeyTypeED25519, "")

	// Try to add the same key again (same base name).
	_, err := mgr.Add(info.Path)
	var target *utils.ErrAlreadyExists
	require.ErrorAs(t, err, &target)
}

func TestKeyManager_Rotate(t *testing.T) {
	mgr := newKeyManager(t)
	info, err := mgr.Generate("rotate_key", config.KeyTypeED25519, "original")
	require.NoError(t, err)
	origPath := info.Path

	newInfo, err := mgr.Rotate("rotate_key", config.KeyTypeED25519, "rotated")
	require.NoError(t, err)
	assert.Equal(t, "rotate_key", newInfo.Name)

	// New key should be in place.
	assert.True(t, utils.FileExists(newInfo.Path))

	// Old key should be backed up.
	assert.True(t, utils.FileExists(origPath+".bak"))
}

func TestKeyManager_Rotate_NotFound(t *testing.T) {
	mgr := newKeyManager(t)
	_, err := mgr.Rotate("ghost", config.KeyTypeED25519, "")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestKeyManager_ListExcludesPubAndBak(t *testing.T) {
	mgr := newKeyManager(t)
	info, err := mgr.Generate("main_key", config.KeyTypeED25519, "")
	require.NoError(t, err)

	// Create a stray .pub and .bak file.
	_ = os.WriteFile(filepath.Join(mgr.KeysDir(), "other.pub"), []byte("ssh-ed25519 xxx"), 0644)
	_ = os.WriteFile(filepath.Join(mgr.KeysDir(), "old_key.bak"), []byte("private"), 0600)
	_ = info

	keys, err := mgr.List()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "main_key", keys[0].Name)
}
