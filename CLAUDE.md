# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

**sshelf** is a Go CLI tool that manages SSH profiles, key pairs, host aliases, and ssh-agent sessions — replacing hand-editing of `~/.ssh/config`. It owns a delimited block inside `~/.ssh/config` and stores its own data in `~/.sshelf/`.

## Commands

```bash
# Build
go build ./cmd/sshelf/

# Run all tests (with race detector)
go test -v -race ./...

# Run a single test file
go test -v -race ./internal/core/ -run TestProfileStore

# Lint (requires golangci-lint installed)
golangci-lint run

# Verify module integrity + tidy
go mod verify && go mod tidy

# Version injection at build time
go build -ldflags="-X github.com/ajf1016/sshelf/internal/config.AppVersion=v1.2.3" ./cmd/sshelf/
```

## Architecture

The code is split into three layers:

**`internal/commands/`** — Cobra command handlers. Thin wrappers only: parse args/flags, call core services, format output with `lipgloss`. No business logic.

**`internal/core/`** — All business logic:
- `store.go` — `Store` is initialized once at startup (`getApp()` in `root.go`) and shared across all commands. It resolves paths for all sub-resources.
- `profile.go` — `ProfileStore`: CRUD on `~/.sshelf/profiles.toml` (BurntSushi/toml). Profile `Name` is the TOML map key, never stored in the file body.
- `host.go` — `HostStore`: same pattern as ProfileStore but for `~/.sshelf/hosts.toml`.
- `key.go` — `KeyManager`: generates ed25519/RSA pairs via `os/exec → ssh-keygen`. Keys stored in `~/.sshelf/keys/`.
- `sshconfig.go` — `SSHConfigWriter`: reads `~/.ssh/config`, replaces only the `# BEGIN sshelf-managed` / `# END sshelf-managed` delimited block, then atomically writes back (`write temp → os.Rename`). Content outside the block is never touched.
- `agent.go` — `AgentManager`: spawns `ssh-agent`, persists socket/PID to `~/.sshelf/agent.env`. Platform-specific fork in `agent_unix.go` / `agent_windows.go`.
- `doctor.go` — `DoctorRunner`: independent check functions each returning `(status, message)`. Output rendered as a lipgloss table.

**`internal/wizard/`** — Interactive TUI flows using `charmbracelet/huh`. Returns fully-formed structs passed to core services.

**`internal/config/constants.go`** — All constants: file names, SSH config delimiters, key types, profile types, permission bits, and path helpers (`DefaultConfigDir`, `SSHConfigPath`).

**`internal/utils/`** — `fs.go` has `AtomicWrite` (used by every store save), `EnsureDir`, `FileExists`. `errors.go` has typed errors: `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidInput`.

## Key design constraints

- `appState` in `root.go` is initialized lazily on first `getApp()` call so commands like `completion` never touch the filesystem.
- Every store write goes through `utils.AtomicWrite` (temp file + `os.Rename`) to prevent corrupt state on crash.
- `SSHConfigWriter.Sync` is idempotent — calling it twice with the same input produces the same file.
- Profile switching requires `eval $(sshelf profile switch <name>)` in the parent shell because subprocesses cannot export env vars to the parent.
- `gosec G204` (subprocess launched with variable) is explicitly excluded in `.golangci.yml` — `ssh-keygen` calls via `os/exec` are intentional.
- Key files stored without passphrases; security relies on `0600` filesystem permissions enforced at generation and checked by `doctor`.

## Data files

| File | Purpose |
|---|---|
| `~/.sshelf/profiles.toml` | Profile store — TOML map keyed by profile name |
| `~/.sshelf/hosts.toml` | Host alias store — same pattern |
| `~/.sshelf/keys/` | SSH key pairs (`<name>_ed25519`, `<name>_ed25519.pub`) |
| `~/.sshelf/agent.env` | ssh-agent socket path + PID |
| `~/.ssh/config` | Written atomically; only the managed block is changed |
