package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ajf1016/sshelf/internal/config"
	"github.com/ajf1016/sshelf/internal/utils"
)

// AgentInfo represents the observable state of an ssh-agent process.
type AgentInfo struct {
	PID        int
	SocketPath string
	Running    bool
}

// AgentManager manages the lifecycle of a single ssh-agent process.
// State (PID + socket path) is persisted to an env file for cross-session access.
type AgentManager struct {
	envPath string
}

// NewAgentManager returns a manager that persists agent state at envPath.
func NewAgentManager(envPath string) *AgentManager {
	return &AgentManager{envPath: envPath}
}

// Start spawns a new ssh-agent and writes its socket/PID to the env file.
// Returns ErrAlreadyExists if an agent is already running.
func (m *AgentManager) Start() (*AgentInfo, error) {
	if info, err := m.Status(); err == nil && info.Running {
		return info, &utils.ErrAlreadyExists{Resource: "ssh-agent"}
	}

	out, err := exec.Command("ssh-agent", "-s").Output()
	if err != nil {
		return nil, fmt.Errorf("spawn ssh-agent: %w", err)
	}

	info, err := parseAgentOutput(string(out))
	if err != nil {
		return nil, err
	}
	if err := m.writeEnv(info); err != nil {
		return nil, err
	}
	return info, nil
}

// Stop terminates the running agent and removes the env file.
func (m *AgentManager) Stop() error {
	info, err := m.Status()
	if err != nil || !info.Running {
		return &utils.ErrNotFound{Resource: "running ssh-agent"}
	}

	// Prefer ssh-agent -k for clean shutdown (removes the socket file).
	cmd := exec.Command("ssh-agent", "-k")
	cmd.Env = append(os.Environ(),
		"SSH_AUTH_SOCK="+info.SocketPath,
		"SSH_AGENT_PID="+strconv.Itoa(info.PID),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-agent -k: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	_ = os.Remove(m.envPath)
	return nil
}

// Status reads the env file and checks whether the agent process is alive.
func (m *AgentManager) Status() (*AgentInfo, error) {
	info, err := m.readEnv()
	if err != nil {
		return &AgentInfo{}, err
	}
	info.Running = isProcessAlive(info.PID)
	return info, nil
}

// LoadKey adds a key to the running agent identified by socketPath.
func (m *AgentManager) LoadKey(keyPath, socketPath string) error {
	cmd := exec.Command("ssh-add", keyPath)
	cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+socketPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-add %s: %w\n%s", filepath.Base(keyPath), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListKeys returns the lines output by "ssh-add -l" for the given socket.
// Returns nil (not an error) when the agent is running but has no keys loaded.
func (m *AgentManager) ListKeys(socketPath string) ([]string, error) {
	cmd := exec.Command("ssh-add", "-l")
	cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+socketPath)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// Exit code 1 means "no identities" — not a real error.
			return nil, nil
		}
		return nil, fmt.Errorf("ssh-add -l: %w", err)
	}

	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// Reload stops the current agent (if running) and starts a fresh one, then
// loads all keys from keysDir. Returns the new AgentInfo.
func (m *AgentManager) Reload(keysDir string) (*AgentInfo, error) {
	// Ignore stop errors — agent may already be dead.
	_ = m.Stop()

	info, err := m.Start()
	if err != nil {
		// ErrAlreadyExists from Start after a failed Stop: read current state.
		var already *utils.ErrAlreadyExists
		if !errors.As(err, &already) {
			return nil, err
		}
		info, err = m.Status()
		if err != nil {
			return nil, err
		}
	}

	// Load every private key found in keysDir.
	entries, err := os.ReadDir(keysDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read keys dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".pub") || strings.HasSuffix(name, ".bak") || strings.HasPrefix(name, ".") {
			continue
		}
		// Best-effort: skip keys that fail to load (e.g., protected with passphrase).
		_ = m.LoadKey(filepath.Join(keysDir, name), info.SocketPath)
	}
	return info, nil
}

// EnvLines returns shell export statements suitable for eval.
func (info *AgentInfo) EnvLines() []string {
	return []string{
		fmt.Sprintf("export SSH_AUTH_SOCK=%s", info.SocketPath),
		fmt.Sprintf("export SSH_AGENT_PID=%d", info.PID),
	}
}

func (m *AgentManager) writeEnv(info *AgentInfo) error {
	content := fmt.Sprintf("SSH_AUTH_SOCK=%s\nSSH_AGENT_PID=%d\n", info.SocketPath, info.PID)
	return utils.AtomicWrite(m.envPath, []byte(content), config.ConfigFilePerm)
}

func (m *AgentManager) readEnv() (*AgentInfo, error) {
	if !utils.FileExists(m.envPath) {
		return &AgentInfo{}, &utils.ErrNotFound{Resource: "agent env file"}
	}
	data, err := os.ReadFile(m.envPath)
	if err != nil {
		return nil, fmt.Errorf("read agent env: %w", err)
	}

	info := &AgentInfo{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "SSH_AUTH_SOCK":
			info.SocketPath = strings.TrimSpace(parts[1])
		case "SSH_AGENT_PID":
			pid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil {
				info.PID = pid
			}
		}
	}
	return info, nil
}

func parseAgentOutput(output string) (*AgentInfo, error) {
	info := &AgentInfo{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SSH_AUTH_SOCK="):
			// Format: SSH_AUTH_SOCK=/tmp/ssh-xxx/agent.N; export SSH_AUTH_SOCK;
			val := strings.TrimPrefix(line, "SSH_AUTH_SOCK=")
			if idx := strings.Index(val, ";"); idx != -1 {
				val = val[:idx]
			}
			info.SocketPath = strings.TrimSpace(val)

		case strings.HasPrefix(line, "SSH_AGENT_PID="):
			val := strings.TrimPrefix(line, "SSH_AGENT_PID=")
			if idx := strings.Index(val, ";"); idx != -1 {
				val = val[:idx]
			}
			pid, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return nil, fmt.Errorf("parse agent PID: %w", err)
			}
			info.PID = pid
		}
	}
	if info.SocketPath == "" || info.PID == 0 {
		return nil, fmt.Errorf("failed to parse ssh-agent output")
	}
	info.Running = true
	return info, nil
}
