package core

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// Profile represents a named SSH identity configuration.
type Profile struct {
	// Name is the profile identifier. It is derived from the TOML map key
	// and is never written to the file itself.
	Name      string    `toml:"-"`
	Type      string    `toml:"type"`
	Platform  string    `toml:"platform,omitempty"`
	Email     string    `toml:"email"`
	Username  string    `toml:"username"`
	KeyName   string    `toml:"key_name"`
	Active    bool      `toml:"active"`
	CreatedAt time.Time `toml:"created_at"`
}

// Validate checks that all required fields are present and contain valid values.
func (p *Profile) Validate() error {
	if p.Name == "" {
		return &utils.ErrInvalidInput{Field: "name", Message: "must not be empty"}
	}
	validTypes := map[string]bool{
		config.ProfileTypeGit:    true,
		config.ProfileTypeServer: true,
		config.ProfileTypeClient: true,
	}
	if !validTypes[p.Type] {
		return &utils.ErrInvalidInput{
			Field:   "type",
			Message: fmt.Sprintf("must be one of: %s, %s, %s", config.ProfileTypeGit, config.ProfileTypeServer, config.ProfileTypeClient),
		}
	}
	if p.Email == "" {
		return &utils.ErrInvalidInput{Field: "email", Message: "must not be empty"}
	}
	if p.Username == "" {
		return &utils.ErrInvalidInput{Field: "username", Message: "must not be empty"}
	}
	if p.KeyName == "" {
		return &utils.ErrInvalidInput{Field: "key_name", Message: "must not be empty"}
	}
	return nil
}

// profilesFile is the top-level structure decoded from / encoded to profiles.toml.
type profilesFile struct {
	Profiles map[string]*Profile `toml:"profiles"`
}

// ProfileStore manages CRUD operations on profiles.toml.
type ProfileStore struct {
	path string
}

// NewProfileStore returns a ProfileStore backed by the file at path.
func NewProfileStore(path string) *ProfileStore {
	return &ProfileStore{path: path}
}

// List returns all profiles sorted alphabetically by name.
func (s *ProfileStore) List() ([]*Profile, error) {
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	profiles := make([]*Profile, 0, len(f.Profiles))
	for _, p := range f.Profiles {
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

// Get returns the profile with the given name, or ErrNotFound.
func (s *ProfileStore) Get(name string) (*Profile, error) {
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	p, ok := f.Profiles[name]
	if !ok {
		return nil, &utils.ErrNotFound{Resource: "profile", Name: name}
	}
	return p, nil
}

// Create writes a new profile to disk. Returns ErrAlreadyExists if the name
// is already taken, or ErrInvalidInput if validation fails.
func (s *ProfileStore) Create(p *Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Profiles[p.Name]; ok {
		return &utils.ErrAlreadyExists{Resource: "profile", Name: p.Name}
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	f.Profiles[p.Name] = p
	return s.save(f)
}

// Update overwrites an existing profile. Returns ErrNotFound if the profile
// does not exist, or ErrInvalidInput if validation fails.
func (s *ProfileStore) Update(p *Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Profiles[p.Name]; !ok {
		return &utils.ErrNotFound{Resource: "profile", Name: p.Name}
	}
	f.Profiles[p.Name] = p
	return s.save(f)
}

// Delete removes a profile by name. Returns ErrNotFound if it does not exist.
func (s *ProfileStore) Delete(name string) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Profiles[name]; !ok {
		return &utils.ErrNotFound{Resource: "profile", Name: name}
	}
	delete(f.Profiles, name)
	return s.save(f)
}

// SetActive marks the named profile as active and deactivates all others.
// Returns ErrNotFound if the profile does not exist.
func (s *ProfileStore) SetActive(name string) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Profiles[name]; !ok {
		return &utils.ErrNotFound{Resource: "profile", Name: name}
	}
	for n, p := range f.Profiles {
		p.Active = n == name
	}
	return s.save(f)
}

// ActiveProfile returns the currently active profile. Returns ErrNotFound
// (with an empty Name) if no profile is active.
func (s *ProfileStore) ActiveProfile() (*Profile, error) {
	profiles, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.Active {
			return p, nil
		}
	}
	return nil, &utils.ErrNotFound{Resource: "active profile"}
}

func (s *ProfileStore) load() (*profilesFile, error) {
	f := &profilesFile{Profiles: make(map[string]*Profile)}
	if !utils.FileExists(s.path) {
		return f, nil
	}
	if _, err := toml.DecodeFile(s.path, f); err != nil {
		return nil, fmt.Errorf("decode %s: %w", s.path, err)
	}
	// Populate Name from the map key since it is not stored in the file.
	for name, p := range f.Profiles {
		p.Name = name
	}
	return f, nil
}

func (s *ProfileStore) save(f *profilesFile) error {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = "  "
	if err := enc.Encode(f); err != nil {
		return fmt.Errorf("encode profiles: %w", err)
	}
	return utils.AtomicWrite(s.path, buf.Bytes(), config.ConfigFilePerm)
}
