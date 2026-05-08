package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/core"
)

func newSSHConfigWriter(t *testing.T) (*core.SSHConfigWriter, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	return core.NewSSHConfigWriter(path), path
}

func TestSSHConfigWriter_Sync_CreatesFile(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	hosts := []*core.Host{
		{Name: "gh-work", Hostname: "github.com", User: "git", IdentityKey: "work_ed25519"},
	}
	require.NoError(t, w.Sync(nil, hosts, "/home/user/.sshelf/keys"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, config.SSHConfigBegin)
	assert.Contains(t, content, config.SSHConfigEnd)
	assert.Contains(t, content, "Host gh-work")
	assert.Contains(t, content, "HostName github.com")
	assert.Contains(t, content, "User git")
	assert.Contains(t, content, "IdentityFile")
	assert.Contains(t, content, "work_ed25519")
}

func TestSSHConfigWriter_Sync_Idempotent(t *testing.T) {
	w, path := newSSHConfigWriter(t)
	hosts := []*core.Host{
		{Name: "srv", Hostname: "1.2.3.4", User: "ubuntu", IdentityKey: "srv_key"},
	}

	require.NoError(t, w.Sync(nil, hosts, "/keys"))
	data1, _ := os.ReadFile(path)

	require.NoError(t, w.Sync(nil, hosts, "/keys"))
	data2, _ := os.ReadFile(path)

	assert.Equal(t, string(data1), string(data2), "second sync must produce identical output")
}

func TestSSHConfigWriter_Sync_PreservesExistingContent(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	// Write existing config with a manual host block.
	existing := "Host manual\n  HostName manual.example.com\n  User admin\n"
	require.NoError(t, os.WriteFile(path, []byte(existing), 0600))

	hosts := []*core.Host{
		{Name: "managed", Hostname: "managed.example.com", User: "ubuntu"},
	}
	require.NoError(t, w.Sync(nil, hosts, "/keys"))

	data, _ := os.ReadFile(path)
	content := string(data)

	// Manual block preserved.
	assert.Contains(t, content, "Host manual")
	assert.Contains(t, content, "manual.example.com")
	// Managed block present.
	assert.Contains(t, content, config.SSHConfigBegin)
	assert.Contains(t, content, "Host managed")
}

func TestSSHConfigWriter_Sync_ReplacesExistingBlock(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	before := []*core.Host{{Name: "old-host", Hostname: "old.example.com", User: "root"}}
	require.NoError(t, w.Sync(nil, before, "/keys"))

	after := []*core.Host{{Name: "new-host", Hostname: "new.example.com", User: "admin"}}
	require.NoError(t, w.Sync(nil, after, "/keys"))

	data, _ := os.ReadFile(path)
	content := string(data)

	assert.NotContains(t, content, "old-host")
	assert.Contains(t, content, "new-host")
}

func TestSSHConfigWriter_Remove_DeletesBlock(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	hosts := []*core.Host{{Name: "go-away", Hostname: "example.com", User: "root"}}
	require.NoError(t, w.Sync(nil, hosts, "/keys"))

	require.NoError(t, w.Remove())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, config.SSHConfigBegin)
	assert.NotContains(t, content, config.SSHConfigEnd)
	assert.NotContains(t, content, "go-away")
}

func TestSSHConfigWriter_Remove_PreservesOutsideContent(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	// Existing content before the managed block.
	existing := "Host keep-me\n  HostName keep.example.com\n"
	require.NoError(t, os.WriteFile(path, []byte(existing), 0600))

	hosts := []*core.Host{{Name: "tmp", Hostname: "tmp.example.com", User: "root"}}
	require.NoError(t, w.Sync(nil, hosts, "/keys"))
	require.NoError(t, w.Remove())

	data, _ := os.ReadFile(path)
	content := string(data)
	assert.Contains(t, content, "keep-me")
	assert.NotContains(t, content, config.SSHConfigBegin)
}

func TestSSHConfigWriter_JumpHost(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	hosts := []*core.Host{
		{Name: "bastion", Hostname: "bastion.example.com", User: "ec2-user"},
		{Name: "app", Hostname: "10.0.0.5", User: "ubuntu", JumpHost: "bastion"},
	}
	require.NoError(t, w.Sync(nil, hosts, "/keys"))

	data, _ := os.ReadFile(path)
	content := string(data)
	assert.Contains(t, content, "ProxyJump bastion")
}

func TestSSHConfigWriter_CustomPort(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	hosts := []*core.Host{
		{Name: "custom-port", Hostname: "example.com", User: "user", Port: 2222},
	}
	require.NoError(t, w.Sync(nil, hosts, "/keys"))

	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), "Port 2222")
}

func TestSSHConfigWriter_DefaultPort_NotWritten(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	hosts := []*core.Host{
		{Name: "default-port", Hostname: "example.com", User: "user", Port: 0},
	}
	require.NoError(t, w.Sync(nil, hosts, "/keys"))

	data, _ := os.ReadFile(path)
	assert.NotContains(t, string(data), "Port 22")
}

func TestSSHConfigWriter_ProfileKeyInheritance(t *testing.T) {
	w, path := newSSHConfigWriter(t)

	profiles := []*core.Profile{
		{Name: "work", KeyName: "work_ed25519", Type: "git", Email: "w@e.com", Username: "w"},
	}
	hosts := []*core.Host{
		{Name: "gh-work", Hostname: "github.com", User: "git", ProfileName: "work"},
	}
	require.NoError(t, w.Sync(profiles, hosts, "/home/.sshelf/keys"))

	data, _ := os.ReadFile(path)
	content := string(data)
	assert.Contains(t, content, "work_ed25519")
}

func TestSSHConfigWriter_NeverModifiesBlockCount(t *testing.T) {
	w, path := newSSHConfigWriter(t)
	hosts := []*core.Host{{Name: "h", Hostname: "h.example.com", User: "root"}}

	for i := 0; i < 5; i++ {
		require.NoError(t, w.Sync(nil, hosts, "/keys"))
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	assert.Equal(t, 1, strings.Count(content, config.SSHConfigBegin))
	assert.Equal(t, 1, strings.Count(content, config.SSHConfigEnd))
}
