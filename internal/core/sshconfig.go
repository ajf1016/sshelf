package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// SSHConfigWriter manages the sshelf-owned block inside ~/.ssh/config.
// Content outside the managed block is never modified.
type SSHConfigWriter struct {
	path string
}

// NewSSHConfigWriter returns a writer targeting the SSH config at path.
func NewSSHConfigWriter(path string) *SSHConfigWriter {
	return &SSHConfigWriter{path: path}
}

// Sync regenerates the sshelf-managed block from the given profiles and hosts,
// then atomically writes the result. Content outside the managed block is
// preserved verbatim. Calling Sync twice with the same input is idempotent.
func (w *SSHConfigWriter) Sync(profiles []*Profile, hosts []*Host, keysDir string) error {
	existing, err := w.readExisting()
	if err != nil {
		return err
	}
	block := w.renderBlock(profiles, hosts, keysDir)
	merged := mergeBlock(existing, block)

	sshDir := filepath.Dir(w.path)
	if err := utils.EnsureDir(sshDir, config.DirPerm); err != nil {
		return fmt.Errorf("ensure ssh dir: %w", err)
	}
	return utils.AtomicWrite(w.path, []byte(merged), config.ConfigFilePerm)
}

// Remove strips the sshelf-managed block from the SSH config, leaving all other
// content intact.
func (w *SSHConfigWriter) Remove() error {
	existing, err := w.readExisting()
	if err != nil {
		return err
	}
	merged := mergeBlock(existing, "")
	return utils.AtomicWrite(w.path, []byte(merged), config.ConfigFilePerm)
}

// ParsedHost represents a Host block found in an existing SSH config file.
type ParsedHost struct {
	Alias    string
	Hostname string
	User     string
	Port     string
	KeyFile  string
	Options  map[string]string // all other key-value pairs
}

// ParseExisting reads the SSH config at the writer's path and returns all
// Host blocks that are NOT inside the sshelf-managed block.
func (w *SSHConfigWriter) ParseExisting() ([]*ParsedHost, error) {
	if !utils.FileExists(w.path) {
		return nil, nil
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		return nil, fmt.Errorf("read ssh config: %w", err)
	}
	return parseSSHConfig(string(data), true), nil
}

func (w *SSHConfigWriter) readExisting() (string, error) {
	if !utils.FileExists(w.path) {
		return "", nil
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		return "", fmt.Errorf("read ssh config: %w", err)
	}
	return string(data), nil
}

// mergeBlock replaces the sshelf-managed block inside existing with newBlock.
// If newBlock is empty the block is removed. If no block exists, newBlock is appended.
func mergeBlock(existing, newBlock string) string {
	beginIdx := strings.Index(existing, config.SSHConfigBegin)
	endIdx := strings.Index(existing, config.SSHConfigEnd)

	if beginIdx == -1 || endIdx == -1 {
		if newBlock == "" {
			return existing
		}
		sep := ""
		if existing != "" {
			if !strings.HasSuffix(existing, "\n") {
				existing += "\n"
			}
			sep = "\n"
		}
		return existing + sep + newBlock + "\n"
	}

	before := strings.TrimRight(existing[:beginIdx], " \t\n")
	after := strings.TrimLeft(existing[endIdx+len(config.SSHConfigEnd):], "\n")

	if newBlock == "" {
		if before == "" && after == "" {
			return ""
		}
		if before == "" {
			return after
		}
		if after == "" {
			return before + "\n"
		}
		return before + "\n\n" + after
	}

	var buf strings.Builder
	if before != "" {
		buf.WriteString(before)
		buf.WriteString("\n\n")
	}
	buf.WriteString(newBlock)
	buf.WriteString("\n")
	if after != "" {
		buf.WriteString("\n")
		buf.WriteString(after)
	}
	return buf.String()
}

// renderBlock produces the full managed block content from profiles + hosts.
func (w *SSHConfigWriter) renderBlock(profiles []*Profile, hosts []*Host, keysDir string) string {
	var buf bytes.Buffer
	buf.WriteString(config.SSHConfigBegin)
	buf.WriteString("\n")

	// Map profile name → key path for quick lookup.
	profileKey := make(map[string]string, len(profiles))
	for _, p := range profiles {
		if p.KeyName != "" {
			profileKey[p.Name] = filepath.Join(keysDir, p.KeyName)
		}
	}

	for _, h := range hosts {
		fmt.Fprintf(&buf, "\nHost %s\n", h.Name)
		fmt.Fprintf(&buf, "  HostName %s\n", h.Hostname)
		fmt.Fprintf(&buf, "  User %s\n", h.User)

		if port := h.SSHPort(); port != config.DefaultSSHPort {
			fmt.Fprintf(&buf, "  Port %d\n", port)
		}

		// Resolve identity: explicit key > profile key.
		identityFile := ""
		switch {
		case h.IdentityKey != "":
			identityFile = filepath.Join(keysDir, h.IdentityKey)
		case h.ProfileName != "":
			identityFile = profileKey[h.ProfileName]
		}
		if identityFile != "" {
			fmt.Fprintf(&buf, "  IdentityFile %s\n", identityFile)
		}

		if h.JumpHost != "" {
			fmt.Fprintf(&buf, "  ProxyJump %s\n", h.JumpHost)
		}
	}

	buf.WriteString("\n")
	buf.WriteString(config.SSHConfigEnd)
	return buf.String()
}

// parseSSHConfig parses SSH config text into ParsedHost blocks.
// When skipManaged is true, the sshelf-managed block is excluded.
func parseSSHConfig(text string, skipManaged bool) []*ParsedHost {
	var hosts []*ParsedHost
	var current *ParsedHost
	inManaged := false

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)

		// Track managed block boundaries.
		if strings.Contains(line, config.SSHConfigBegin) {
			inManaged = true
			continue
		}
		if strings.Contains(line, config.SSHConfigEnd) {
			inManaged = false
			continue
		}
		if skipManaged && inManaged {
			continue
		}

		// Skip blanks and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		val := strings.TrimSpace(parts[1])

		if key == "host" {
			if current != nil {
				hosts = append(hosts, current)
			}
			current = &ParsedHost{Alias: val, Options: make(map[string]string)}
			continue
		}
		if current == nil {
			continue
		}

		switch key {
		case "hostname":
			current.Hostname = val
		case "user":
			current.User = val
		case "port":
			current.Port = val
		case "identityfile":
			current.KeyFile = val
		default:
			current.Options[key] = val
		}
	}
	if current != nil {
		hosts = append(hosts, current)
	}
	return hosts
}
