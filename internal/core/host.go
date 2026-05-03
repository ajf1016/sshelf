package core

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/BurntSushi/toml"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// Host represents a named SSH host alias written into the sshelf-managed
// block of ~/.ssh/config.
type Host struct {
	// Name is the SSH Host alias and the TOML map key. Never written to the file.
	Name        string `toml:"-"`
	Hostname    string `toml:"hostname"`
	User        string `toml:"user"`
	Port        int    `toml:"port,omitempty"`
	IdentityKey string `toml:"identity_key"`
	ProfileName string `toml:"profile,omitempty"`
	JumpHost    string `toml:"jump_host,omitempty"`
}

// Validate checks that all required fields are present and contain valid values.
func (h *Host) Validate() error {
	if h.Name == "" {
		return &utils.ErrInvalidInput{Field: "name", Message: "must not be empty"}
	}
	if h.Hostname == "" {
		return &utils.ErrInvalidInput{Field: "hostname", Message: "must not be empty"}
	}
	if h.User == "" {
		return &utils.ErrInvalidInput{Field: "user", Message: "must not be empty"}
	}
	if h.Port != 0 && (h.Port < 1 || h.Port > 65535) {
		return &utils.ErrInvalidInput{Field: "port", Message: "must be between 1 and 65535"}
	}
	return nil
}

// SSHPort returns the effective SSH port, falling back to DefaultSSHPort when
// none is explicitly set.
func (h *Host) SSHPort() int {
	if h.Port == 0 {
		return config.DefaultSSHPort
	}
	return h.Port
}

// hostsFile is the top-level structure decoded from / encoded to hosts.toml.
type hostsFile struct {
	Hosts map[string]*Host `toml:"hosts"`
}

// HostStore manages CRUD operations on hosts.toml.
type HostStore struct {
	path string
}

// NewHostStore returns a HostStore backed by the file at path.
func NewHostStore(path string) *HostStore {
	return &HostStore{path: path}
}

// List returns all hosts sorted alphabetically by name.
func (s *HostStore) List() ([]*Host, error) {
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	hosts := make([]*Host, 0, len(f.Hosts))
	for _, h := range f.Hosts {
		hosts = append(hosts, h)
	}
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Name < hosts[j].Name
	})
	return hosts, nil
}

// ListByProfile returns all hosts associated with the given profile name,
// sorted alphabetically.
func (s *HostStore) ListByProfile(profileName string) ([]*Host, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	filtered := make([]*Host, 0)
	for _, h := range all {
		if h.ProfileName == profileName {
			filtered = append(filtered, h)
		}
	}
	return filtered, nil
}

// Get returns the host with the given name, or ErrNotFound.
func (s *HostStore) Get(name string) (*Host, error) {
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	h, ok := f.Hosts[name]
	if !ok {
		return nil, &utils.ErrNotFound{Resource: "host", Name: name}
	}
	return h, nil
}

// Create writes a new host alias to disk. Returns ErrAlreadyExists if the
// name is taken, or ErrInvalidInput if validation fails.
func (s *HostStore) Create(h *Host) error {
	if err := h.Validate(); err != nil {
		return err
	}
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Hosts[h.Name]; ok {
		return &utils.ErrAlreadyExists{Resource: "host", Name: h.Name}
	}
	f.Hosts[h.Name] = h
	return s.save(f)
}

// Update overwrites an existing host. Returns ErrNotFound if the host does
// not exist, or ErrInvalidInput if validation fails.
func (s *HostStore) Update(h *Host) error {
	if err := h.Validate(); err != nil {
		return err
	}
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Hosts[h.Name]; !ok {
		return &utils.ErrNotFound{Resource: "host", Name: h.Name}
	}
	f.Hosts[h.Name] = h
	return s.save(f)
}

// Delete removes a host by name. Returns ErrNotFound if it does not exist.
func (s *HostStore) Delete(name string) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := f.Hosts[name]; !ok {
		return &utils.ErrNotFound{Resource: "host", Name: name}
	}
	delete(f.Hosts, name)
	return s.save(f)
}

func (s *HostStore) load() (*hostsFile, error) {
	f := &hostsFile{Hosts: make(map[string]*Host)}
	if !utils.FileExists(s.path) {
		return f, nil
	}
	if _, err := toml.DecodeFile(s.path, f); err != nil {
		return nil, fmt.Errorf("decode %s: %w", s.path, err)
	}
	// Populate Name from the map key since it is not stored in the file.
	for name, h := range f.Hosts {
		h.Name = name
	}
	return f, nil
}

func (s *HostStore) save(f *hostsFile) error {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = "  "
	if err := enc.Encode(f); err != nil {
		return fmt.Errorf("encode hosts: %w", err)
	}
	return utils.AtomicWrite(s.path, buf.Bytes(), config.ConfigFilePerm)
}
