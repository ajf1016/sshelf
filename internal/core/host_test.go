package core_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/utils"
)

func newTestHost(name string) *core.Host {
	return &core.Host{
		Name:        name,
		Hostname:    name + ".example.com",
		User:        "ubuntu",
		IdentityKey: "work_ed25519",
		ProfileName: "work",
	}
}

func newHostStore(t *testing.T) *core.HostStore {
	t.Helper()
	return core.NewHostStore(filepath.Join(t.TempDir(), "hosts.toml"))
}

func TestHostStore_CreateAndGet(t *testing.T) {
	store := newHostStore(t)

	h := newTestHost("staging")
	require.NoError(t, store.Create(h))

	got, err := store.Get("staging")
	require.NoError(t, err)
	assert.Equal(t, "staging", got.Name)
	assert.Equal(t, "staging.example.com", got.Hostname)
	assert.Equal(t, "ubuntu", got.User)
}

func TestHostStore_CreateDuplicate(t *testing.T) {
	store := newHostStore(t)
	require.NoError(t, store.Create(newTestHost("staging")))

	err := store.Create(newTestHost("staging"))

	var target *utils.ErrAlreadyExists
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "host", target.Resource)
}

func TestHostStore_GetNotFound(t *testing.T) {
	store := newHostStore(t)

	_, err := store.Get("ghost")

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "host", target.Resource)
}

func TestHostStore_List_SortedAlphabetically(t *testing.T) {
	store := newHostStore(t)
	for _, n := range []string{"prod", "staging", "dev"} {
		require.NoError(t, store.Create(newTestHost(n)))
	}

	hosts, err := store.List()
	require.NoError(t, err)
	require.Len(t, hosts, 3)
	assert.Equal(t, "dev", hosts[0].Name)
	assert.Equal(t, "prod", hosts[1].Name)
	assert.Equal(t, "staging", hosts[2].Name)
}

func TestHostStore_ListByProfile(t *testing.T) {
	store := newHostStore(t)

	workHost := newTestHost("github-work")
	workHost.ProfileName = "work"
	require.NoError(t, store.Create(workHost))

	personalHost := newTestHost("github-personal")
	personalHost.ProfileName = "personal"
	require.NoError(t, store.Create(personalHost))

	workHosts, err := store.ListByProfile("work")
	require.NoError(t, err)
	require.Len(t, workHosts, 1)
	assert.Equal(t, "github-work", workHosts[0].Name)

	personalHosts, err := store.ListByProfile("personal")
	require.NoError(t, err)
	require.Len(t, personalHosts, 1)
	assert.Equal(t, "github-personal", personalHosts[0].Name)
}

func TestHostStore_ListByProfile_Empty(t *testing.T) {
	store := newHostStore(t)

	hosts, err := store.ListByProfile("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, hosts)
}

func TestHostStore_Update(t *testing.T) {
	store := newHostStore(t)
	h := newTestHost("staging")
	require.NoError(t, store.Create(h))

	h.User = "ec2-user"
	require.NoError(t, store.Update(h))

	got, err := store.Get("staging")
	require.NoError(t, err)
	assert.Equal(t, "ec2-user", got.User)
}

func TestHostStore_Update_NotFound(t *testing.T) {
	store := newHostStore(t)

	err := store.Update(newTestHost("ghost"))

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestHostStore_Delete(t *testing.T) {
	store := newHostStore(t)
	require.NoError(t, store.Create(newTestHost("staging")))
	require.NoError(t, store.Delete("staging"))

	_, err := store.Get("staging")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestHostStore_Delete_NotFound(t *testing.T) {
	store := newHostStore(t)

	err := store.Delete("ghost")

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestHostStore_SSHPort_DefaultsTo22(t *testing.T) {
	h := &core.Host{Port: 0}
	assert.Equal(t, 22, h.SSHPort())
}

func TestHostStore_SSHPort_Custom(t *testing.T) {
	h := &core.Host{Port: 2222}
	assert.Equal(t, 2222, h.SSHPort())
}

func TestHostStore_Validate_RequiredFields(t *testing.T) {
	store := newHostStore(t)

	cases := []struct {
		desc  string
		host  *core.Host
		field string
	}{
		{
			desc:  "empty name",
			host:  &core.Host{Hostname: "h", User: "u"},
			field: "name",
		},
		{
			desc:  "empty hostname",
			host:  &core.Host{Name: "x", User: "u"},
			field: "hostname",
		},
		{
			desc:  "empty user",
			host:  &core.Host{Name: "x", Hostname: "h"},
			field: "user",
		},
		{
			desc:  "port out of range",
			host:  &core.Host{Name: "x", Hostname: "h", User: "u", Port: 99999},
			field: "port",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := store.Create(tc.host)
			var target *utils.ErrInvalidInput
			require.ErrorAs(t, err, &target)
			assert.Equal(t, tc.field, target.Field)
		})
	}
}

func TestHostStore_JumpHost_RoundTrip(t *testing.T) {
	store := newHostStore(t)

	h := newTestHost("prod-app")
	h.JumpHost = "bastion"
	require.NoError(t, store.Create(h))

	got, err := store.Get("prod-app")
	require.NoError(t, err)
	assert.Equal(t, "bastion", got.JumpHost)
}

func TestHostStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.toml")

	store1 := core.NewHostStore(path)
	require.NoError(t, store1.Create(newTestHost("staging")))

	// A fresh store instance over the same file must see the written host.
	store2 := core.NewHostStore(path)
	got, err := store2.Get("staging")
	require.NoError(t, err)
	assert.Equal(t, "staging.example.com", got.Hostname)
}
