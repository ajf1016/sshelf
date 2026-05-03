package core_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajf1016/sshelf/internal/core"
	"github.com/ajf1016/sshelf/internal/utils"
)

func newTestProfile(name string) *core.Profile {
	return &core.Profile{
		Name:     name,
		Type:     "git",
		Platform: "github",
		Email:    name + "@example.com",
		Username: name + "-user",
		KeyName:  name + "_ed25519",
	}
}

func newProfileStore(t *testing.T) *core.ProfileStore {
	t.Helper()
	return core.NewProfileStore(filepath.Join(t.TempDir(), "profiles.toml"))
}

func TestProfileStore_CreateAndGet(t *testing.T) {
	store := newProfileStore(t)

	p := newTestProfile("work")
	require.NoError(t, store.Create(p))

	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "work", got.Name)
	assert.Equal(t, "git", got.Type)
	assert.Equal(t, "work@example.com", got.Email)
	assert.WithinDuration(t, time.Now(), got.CreatedAt, 5*time.Second)
}

func TestProfileStore_CreateDuplicate(t *testing.T) {
	store := newProfileStore(t)
	require.NoError(t, store.Create(newTestProfile("work")))

	err := store.Create(newTestProfile("work"))

	var target *utils.ErrAlreadyExists
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "profile", target.Resource)
	assert.Equal(t, "work", target.Name)
}

func TestProfileStore_GetNotFound(t *testing.T) {
	store := newProfileStore(t)

	_, err := store.Get("ghost")

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
	assert.Equal(t, "profile", target.Resource)
	assert.Equal(t, "ghost", target.Name)
}

func TestProfileStore_List_SortedAlphabetically(t *testing.T) {
	store := newProfileStore(t)
	for _, n := range []string{"zebra", "alpha", "mango"} {
		require.NoError(t, store.Create(newTestProfile(n)))
	}

	profiles, err := store.List()
	require.NoError(t, err)
	require.Len(t, profiles, 3)
	assert.Equal(t, "alpha", profiles[0].Name)
	assert.Equal(t, "mango", profiles[1].Name)
	assert.Equal(t, "zebra", profiles[2].Name)
}

func TestProfileStore_Update(t *testing.T) {
	store := newProfileStore(t)
	p := newTestProfile("work")
	require.NoError(t, store.Create(p))

	p.Email = "updated@example.com"
	require.NoError(t, store.Update(p))

	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "updated@example.com", got.Email)
}

func TestProfileStore_Update_NotFound(t *testing.T) {
	store := newProfileStore(t)

	err := store.Update(newTestProfile("ghost"))

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestProfileStore_Delete(t *testing.T) {
	store := newProfileStore(t)
	require.NoError(t, store.Create(newTestProfile("work")))
	require.NoError(t, store.Delete("work"))

	_, err := store.Get("work")
	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestProfileStore_Delete_NotFound(t *testing.T) {
	store := newProfileStore(t)

	err := store.Delete("ghost")

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestProfileStore_SetActive_ExclusiveActivation(t *testing.T) {
	store := newProfileStore(t)
	require.NoError(t, store.Create(newTestProfile("work")))
	require.NoError(t, store.Create(newTestProfile("personal")))

	require.NoError(t, store.SetActive("work"))
	work, err := store.Get("work")
	require.NoError(t, err)
	assert.True(t, work.Active)

	personal, err := store.Get("personal")
	require.NoError(t, err)
	assert.False(t, personal.Active)

	// Switching deactivates the previous active profile.
	require.NoError(t, store.SetActive("personal"))
	work, err = store.Get("work")
	require.NoError(t, err)
	assert.False(t, work.Active)
	personal, err = store.Get("personal")
	require.NoError(t, err)
	assert.True(t, personal.Active)
}

func TestProfileStore_ActiveProfile(t *testing.T) {
	store := newProfileStore(t)
	require.NoError(t, store.Create(newTestProfile("work")))
	require.NoError(t, store.SetActive("work"))

	active, err := store.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active.Name)
}

func TestProfileStore_ActiveProfile_NoneSet(t *testing.T) {
	store := newProfileStore(t)
	require.NoError(t, store.Create(newTestProfile("work")))

	_, err := store.ActiveProfile()

	var target *utils.ErrNotFound
	require.ErrorAs(t, err, &target)
}

func TestProfileStore_Validate_RequiredFields(t *testing.T) {
	store := newProfileStore(t)

	cases := []struct {
		desc    string
		profile *core.Profile
		field   string
	}{
		{
			desc:    "empty name",
			profile: &core.Profile{Type: "git", Email: "a@b.com", Username: "u", KeyName: "k"},
			field:   "name",
		},
		{
			desc:    "invalid type",
			profile: &core.Profile{Name: "x", Type: "ftp", Email: "a@b.com", Username: "u", KeyName: "k"},
			field:   "type",
		},
		{
			desc:    "empty email",
			profile: &core.Profile{Name: "x", Type: "git", Username: "u", KeyName: "k"},
			field:   "email",
		},
		{
			desc:    "empty username",
			profile: &core.Profile{Name: "x", Type: "git", Email: "a@b.com", KeyName: "k"},
			field:   "username",
		},
		{
			desc:    "empty key_name",
			profile: &core.Profile{Name: "x", Type: "git", Email: "a@b.com", Username: "u"},
			field:   "key_name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := store.Create(tc.profile)
			var target *utils.ErrInvalidInput
			require.ErrorAs(t, err, &target)
			assert.Equal(t, tc.field, target.Field)
		})
	}
}

func TestProfileStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")

	store1 := core.NewProfileStore(path)
	require.NoError(t, store1.Create(newTestProfile("work")))

	// A fresh store instance over the same file must see the written profile.
	store2 := core.NewProfileStore(path)
	got, err := store2.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "work@example.com", got.Email)
}
