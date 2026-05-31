package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// KeyInfo holds metadata about a managed SSH key pair.
type KeyInfo struct {
	Name      string    // base filename without extension
	Type      string    // "ed25519" or "rsa"
	Comment   string    // comment embedded in the public key
	CreatedAt time.Time // derived from file mtime
	Path      string    // absolute path to the private key file
}

// PublicKeyPath returns the path to the corresponding .pub file.
func (k *KeyInfo) PublicKeyPath() string { return k.Path + ".pub" }

// Age returns the duration elapsed since the key was created.
func (k *KeyInfo) Age() time.Duration { return time.Since(k.CreatedAt) }

// KeyManager manages SSH key pairs stored under the sshelf keys directory.
type KeyManager struct {
	keysDir string
}

// NewKeyManager returns a KeyManager pointing at keysDir.
func NewKeyManager(keysDir string) *KeyManager {
	return &KeyManager{keysDir: keysDir}
}

// KeysDir returns the managed keys directory path.
func (m *KeyManager) KeysDir() string { return m.keysDir }

// Generate creates a new key pair in the keys directory.
// keyType must be config.KeyTypeED25519 or config.KeyTypeRSA.
// An empty passphrase is used (keys protected by filesystem permissions).
func (m *KeyManager) Generate(name, keyType, comment string) (*KeyInfo, error) {
	if name == "" {
		return nil, &utils.ErrInvalidInput{Field: "name", Message: "must not be empty"}
	}
	if keyType != config.KeyTypeED25519 && keyType != config.KeyTypeRSA {
		return nil, &utils.ErrInvalidInput{
			Field:   "key_type",
			Message: fmt.Sprintf("must be %q or %q", config.KeyTypeED25519, config.KeyTypeRSA),
		}
	}

	privPath := filepath.Join(m.keysDir, name)
	if utils.FileExists(privPath) {
		return nil, &utils.ErrAlreadyExists{Resource: "key", Name: name}
	}

	args := []string{"-t", keyType, "-f", privPath, "-N", ""}
	if comment != "" {
		args = append(args, "-C", comment)
	}

	// #nosec G204 — keyType is validated above; name comes from user input via CLI flag
	out, err := exec.Command("ssh-keygen", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ssh-keygen: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	if err := enforceKeyPermissions(privPath); err != nil {
		return nil, err
	}
	return m.infoFromPath(privPath)
}

// Add imports an existing key pair by copying it into the managed keys directory.
// The public key is expected at srcPath+".pub"; missing .pub is tolerated.
func (m *KeyManager) Add(srcPath string) (*KeyInfo, error) {
	if !utils.FileExists(srcPath) {
		return nil, &utils.ErrNotFound{Resource: "key file", Name: srcPath}
	}
	name := filepath.Base(srcPath)
	dstPath := filepath.Join(m.keysDir, name)
	if utils.FileExists(dstPath) {
		return nil, &utils.ErrAlreadyExists{Resource: "key", Name: name}
	}

	privData, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	if err := utils.AtomicWrite(dstPath, privData, config.PrivateKeyPerm); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}

	pubSrc := srcPath + ".pub"
	if utils.FileExists(pubSrc) {
		pubData, err := os.ReadFile(pubSrc)
		if err != nil {
			return nil, fmt.Errorf("read public key: %w", err)
		}
		if err := utils.AtomicWrite(dstPath+".pub", pubData, config.PublicKeyPerm); err != nil {
			return nil, fmt.Errorf("write public key: %w", err)
		}
	}

	return m.infoFromPath(dstPath)
}

// List returns metadata for all managed key pairs sorted alphabetically.
// .pub and .bak files are excluded — only private key entries are returned.
func (m *KeyManager) List() ([]*KeyInfo, error) {
	entries, err := os.ReadDir(m.keysDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read keys dir: %w", err)
	}

	var keys []*KeyInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip .pub files, .bak backups, and hidden files.
		if strings.HasSuffix(name, ".pub") ||
			strings.HasSuffix(name, ".bak") ||
			strings.HasPrefix(name, ".") {
			continue
		}
		info, err := m.infoFromPath(filepath.Join(m.keysDir, name))
		if err != nil {
			continue
		}
		keys = append(keys, info)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Name < keys[j].Name })
	return keys, nil
}

// Get returns the KeyInfo for the named key, or ErrNotFound.
func (m *KeyManager) Get(name string) (*KeyInfo, error) {
	path := filepath.Join(m.keysDir, name)
	if !utils.FileExists(path) {
		return nil, &utils.ErrNotFound{Resource: "key", Name: name}
	}
	return m.infoFromPath(path)
}

// Remove deletes the named private key and its public key.
func (m *KeyManager) Remove(name string) error {
	privPath := filepath.Join(m.keysDir, name)
	if !utils.FileExists(privPath) {
		return &utils.ErrNotFound{Resource: "key", Name: name}
	}
	if err := os.Remove(privPath); err != nil {
		return fmt.Errorf("remove private key: %w", err)
	}
	pubPath := privPath + ".pub"
	if utils.FileExists(pubPath) {
		_ = os.Remove(pubPath)
	}
	return nil
}

// Rotate generates a new key pair, backs up the old one as <name>.bak, then
// renames the new pair to <name>. Returns the new KeyInfo.
func (m *KeyManager) Rotate(name, keyType, comment string) (*KeyInfo, error) {
	oldPriv := filepath.Join(m.keysDir, name)
	if !utils.FileExists(oldPriv) {
		return nil, &utils.ErrNotFound{Resource: "key", Name: name}
	}

	tmpName := name + "_rotating"
	newInfo, err := m.Generate(tmpName, keyType, comment)
	if err != nil {
		return nil, fmt.Errorf("generate replacement key: %w", err)
	}

	// Backup old pair.
	if err := os.Rename(oldPriv, oldPriv+".bak"); err != nil {
		_ = m.Remove(tmpName)
		return nil, fmt.Errorf("backup old private key: %w", err)
	}
	if utils.FileExists(oldPriv + ".pub") {
		_ = os.Rename(oldPriv+".pub", oldPriv+".pub.bak")
	}

	// Move new pair into place.
	if err := os.Rename(newInfo.Path, oldPriv); err != nil {
		return nil, fmt.Errorf("rename new private key: %w", err)
	}
	if utils.FileExists(newInfo.PublicKeyPath()) {
		_ = os.Rename(newInfo.PublicKeyPath(), oldPriv+".pub")
	}

	return m.infoFromPath(oldPriv)
}

// FixPermissions enforces 0600 on the private key and 0644 on the public key.
func (m *KeyManager) FixPermissions(name string) error {
	return enforceKeyPermissions(filepath.Join(m.keysDir, name))
}

// ReadPublicKey returns the trimmed contents of <name>.pub.
func (m *KeyManager) ReadPublicKey(name string) (string, error) {
	path := filepath.Join(m.keysDir, name+".pub")
	if !utils.FileExists(path) {
		return "", &utils.ErrNotFound{Resource: "public key", Name: name}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read public key: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// CheckPermissions returns an error if the key permissions are not 0600/0644.
func (m *KeyManager) CheckPermissions(name string) error {
	privPath := filepath.Join(m.keysDir, name)
	fi, err := os.Stat(privPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", name, err)
	}
	if fi.Mode().Perm() != config.PrivateKeyPerm {
		return fmt.Errorf("private key %s has permissions %v (want %v)",
			name, fi.Mode().Perm(), config.PrivateKeyPerm)
	}
	pubPath := privPath + ".pub"
	if utils.FileExists(pubPath) {
		fi, err = os.Stat(pubPath)
		if err != nil {
			return fmt.Errorf("stat %s.pub: %w", name, err)
		}
		if fi.Mode().Perm() != config.PublicKeyPerm {
			return fmt.Errorf("public key %s.pub has permissions %v (want %v)",
				name, fi.Mode().Perm(), config.PublicKeyPerm)
		}
	}
	return nil
}

func enforceKeyPermissions(privPath string) error {
	if err := os.Chmod(privPath, config.PrivateKeyPerm); err != nil {
		return fmt.Errorf("chmod private key: %w", err)
	}
	pubPath := privPath + ".pub"
	if utils.FileExists(pubPath) {
		if err := os.Chmod(pubPath, config.PublicKeyPerm); err != nil {
			return fmt.Errorf("chmod public key: %w", err)
		}
	}
	return nil
}

func (m *KeyManager) infoFromPath(privPath string) (*KeyInfo, error) {
	fi, err := os.Stat(privPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", privPath, err)
	}

	name := filepath.Base(privPath)
	info := &KeyInfo{
		Name:      name,
		Type:      config.KeyTypeED25519,
		CreatedAt: fi.ModTime(),
		Path:      privPath,
	}

	// Detect key type and read comment from the public key.
	pubPath := privPath + ".pub"
	if utils.FileExists(pubPath) {
		if data, err := os.ReadFile(pubPath); err == nil {
			parts := strings.Fields(strings.TrimSpace(string(data)))
			if len(parts) >= 2 {
				switch parts[0] {
				case "ssh-rsa":
					info.Type = config.KeyTypeRSA
				case "ssh-ed25519":
					info.Type = config.KeyTypeED25519
				default:
					info.Type = parts[0]
				}
				if len(parts) >= 3 {
					info.Comment = parts[2]
				}
			}
		}
	} else if strings.Contains(name, "rsa") {
		info.Type = config.KeyTypeRSA
	}

	return info, nil
}
