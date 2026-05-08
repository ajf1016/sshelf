package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// CheckStatus is the severity level of a single doctor check.
type CheckStatus int

const (
	CheckPass CheckStatus = iota
	CheckWarn
	CheckFail
)

func (s CheckStatus) String() string {
	switch s {
	case CheckPass:
		return "PASS"
	case CheckWarn:
		return "WARN"
	case CheckFail:
		return "FAIL"
	default:
		return "UNKN"
	}
}

// CheckResult is the result of a single doctor check.
type CheckResult struct {
	Name    string
	Status  CheckStatus
	Message string
	Fixable bool
	Fix     func() error // non-nil only when Fixable is true
}

// DoctorRunner runs all health checks against the sshelf environment.
type DoctorRunner struct {
	store    *Store
	profiles *ProfileStore
	hosts    *HostStore
	keys     *KeyManager
	agent    *AgentManager
}

// NewDoctorRunner creates a runner with references to all required services.
func NewDoctorRunner(
	store *Store,
	profiles *ProfileStore,
	hosts *HostStore,
	keys *KeyManager,
	agent *AgentManager,
) *DoctorRunner {
	return &DoctorRunner{
		store:    store,
		profiles: profiles,
		hosts:    hosts,
		keys:     keys,
		agent:    agent,
	}
}

// RunAll executes every check and returns results in a stable order.
func (r *DoctorRunner) RunAll() []*CheckResult {
	fns := []func() *CheckResult{
		r.checkConfigDir,
		r.checkKeyPermissions,
		r.checkAgentRunning,
		r.checkActiveProfile,
		r.checkKeyAge,
		r.checkOrphanedKeys,
	}
	results := make([]*CheckResult, 0, len(fns))
	for _, fn := range fns {
		results = append(results, fn())
	}
	return results
}

// Run executes the single named check. Returns ErrNotFound for unknown names.
func (r *DoctorRunner) Run(name string) (*CheckResult, error) {
	checks := map[string]func() *CheckResult{
		"config-dir":     r.checkConfigDir,
		"permissions":    r.checkKeyPermissions,
		"agent":          r.checkAgentRunning,
		"active-profile": r.checkActiveProfile,
		"key-age":        r.checkKeyAge,
		"orphaned-keys":  r.checkOrphanedKeys,
	}
	fn, ok := checks[name]
	if !ok {
		return nil, &utils.ErrNotFound{Resource: "check", Name: name}
	}
	return fn(), nil
}

// checkConfigDir verifies the config directory exists with correct permissions.
func (r *DoctorRunner) checkConfigDir() *CheckResult {
	dir := r.store.Dir()
	fi, err := os.Stat(dir)
	if err != nil {
		return &CheckResult{
			Name:    "config-dir",
			Status:  CheckFail,
			Message: fmt.Sprintf("%s does not exist", dir),
			Fixable: true,
			Fix: func() error {
				return utils.EnsureDir(dir, config.DirPerm)
			},
		}
	}
	if fi.Mode().Perm() != config.DirPerm {
		return &CheckResult{
			Name:    "config-dir",
			Status:  CheckWarn,
			Message: fmt.Sprintf("%s has permissions %v (want %v)", dir, fi.Mode().Perm(), config.DirPerm),
			Fixable: true,
			Fix: func() error {
				return os.Chmod(dir, config.DirPerm)
			},
		}
	}
	return &CheckResult{Name: "config-dir", Status: CheckPass, Message: dir + " exists with correct permissions"}
}

// checkKeyPermissions ensures every managed private key is 0600 / public key is 0644.
func (r *DoctorRunner) checkKeyPermissions() *CheckResult {
	keys, err := r.keys.List()
	if err != nil {
		return &CheckResult{Name: "permissions", Status: CheckFail, Message: err.Error()}
	}
	if len(keys) == 0 {
		return &CheckResult{Name: "permissions", Status: CheckPass, Message: "no managed keys"}
	}

	var bad []string
	for _, k := range keys {
		if err := r.keys.CheckPermissions(k.Name); err != nil {
			bad = append(bad, k.Name)
		}
	}
	if len(bad) == 0 {
		return &CheckResult{
			Name:    "permissions",
			Status:  CheckPass,
			Message: fmt.Sprintf("all %d keys have correct permissions", len(keys)),
		}
	}
	names := strings.Join(bad, ", ")
	return &CheckResult{
		Name:    "permissions",
		Status:  CheckFail,
		Message: fmt.Sprintf("bad permissions on: %s", names),
		Fixable: true,
		Fix: func() error {
			for _, name := range bad {
				if err := r.keys.FixPermissions(name); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// checkAgentRunning verifies that the persisted ssh-agent is still alive.
func (r *DoctorRunner) checkAgentRunning() *CheckResult {
	info, err := r.agent.Status()
	if err != nil || !info.Running {
		msg := "ssh-agent is not running"
		if info != nil && info.PID != 0 {
			msg = fmt.Sprintf("ssh-agent (PID %d) is no longer alive", info.PID)
		}
		return &CheckResult{Name: "agent", Status: CheckWarn, Message: msg}
	}
	return &CheckResult{
		Name:    "agent",
		Status:  CheckPass,
		Message: fmt.Sprintf("ssh-agent running (PID %d, socket: %s)", info.PID, info.SocketPath),
	}
}

// checkActiveProfile verifies that at least one profile is marked active.
func (r *DoctorRunner) checkActiveProfile() *CheckResult {
	active, err := r.profiles.ActiveProfile()
	if err != nil {
		return &CheckResult{Name: "active-profile", Status: CheckWarn, Message: "no active profile set"}
	}
	return &CheckResult{
		Name:    "active-profile",
		Status:  CheckPass,
		Message: fmt.Sprintf("active profile: %s (%s)", active.Name, active.Email),
	}
}

const keyAgeWarnDays = 365

// checkKeyAge warns when any key is older than keyAgeWarnDays.
func (r *DoctorRunner) checkKeyAge() *CheckResult {
	keys, err := r.keys.List()
	if err != nil {
		return &CheckResult{Name: "key-age", Status: CheckFail, Message: err.Error()}
	}
	if len(keys) == 0 {
		return &CheckResult{Name: "key-age", Status: CheckPass, Message: "no managed keys"}
	}

	threshold := time.Duration(keyAgeWarnDays) * 24 * time.Hour
	var old []string
	for _, k := range keys {
		if k.Age() > threshold {
			days := int(k.Age().Hours() / 24)
			old = append(old, fmt.Sprintf("%s (%d days)", k.Name, days))
		}
	}
	if len(old) == 0 {
		return &CheckResult{
			Name:    "key-age",
			Status:  CheckPass,
			Message: fmt.Sprintf("all keys are younger than %d days", keyAgeWarnDays),
		}
	}
	return &CheckResult{
		Name:    "key-age",
		Status:  CheckWarn,
		Message: fmt.Sprintf("keys older than %d days: %s", keyAgeWarnDays, strings.Join(old, ", ")),
	}
}

// checkOrphanedKeys reports key files in keysDir not referenced by any profile.
func (r *DoctorRunner) checkOrphanedKeys() *CheckResult {
	keys, err := r.keys.List()
	if err != nil {
		return &CheckResult{Name: "orphaned-keys", Status: CheckFail, Message: err.Error()}
	}
	if len(keys) == 0 {
		return &CheckResult{Name: "orphaned-keys", Status: CheckPass, Message: "no managed keys"}
	}

	profiles, err := r.profiles.List()
	if err != nil {
		return &CheckResult{Name: "orphaned-keys", Status: CheckFail, Message: err.Error()}
	}

	referenced := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		referenced[p.KeyName] = true
	}

	var orphans []string
	for _, k := range keys {
		if !referenced[k.Name] {
			orphans = append(orphans, k.Name)
		}
	}

	if len(orphans) == 0 {
		return &CheckResult{Name: "orphaned-keys", Status: CheckPass, Message: "all keys are referenced by a profile"}
	}
	return &CheckResult{
		Name:    "orphaned-keys",
		Status:  CheckWarn,
		Message: fmt.Sprintf("orphaned keys (not linked to any profile): %s", strings.Join(orphans, ", ")),
		Fixable: true,
		Fix: func() error {
			for _, name := range orphans {
				bakPriv := filepath.Join(r.keys.KeysDir(), name+".bak")
				priv := filepath.Join(r.keys.KeysDir(), name)
				if err := os.Rename(priv, bakPriv); err != nil {
					return fmt.Errorf("archive %s: %w", name, err)
				}
				pub := priv + ".pub"
				if utils.FileExists(pub) {
					_ = os.Rename(pub, pub+".bak")
				}
			}
			return nil
		},
	}
}
