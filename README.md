# sshelf

> Manage SSH profiles, keys, host aliases, and agent sessions from a single CLI — no more hand-editing `~/.ssh/config`.

[![CI](https://github.com/ajf1016/sshelf/actions/workflows/ci.yml/badge.svg)](https://github.com/ajf1016/sshelf/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](#installation)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

---

## What is sshelf?

If you're a developer juggling multiple Git accounts (personal, work, freelance clients), remote servers, and cloud environments — all from one machine — you've felt this pain before:

- Googling *"how to use two GitHub accounts with SSH"* every six months
- Keeping a graveyard of `id_rsa`, `id_rsa_2`, `key_work.pem` files in `~/.ssh/` with no idea what they're for
- Manually running `ssh-add` after every reboot
- Breaking your SSH config with a stray edit

**sshelf fixes all of that.** It gives each identity (a *profile*) its own named key pair, tracks host aliases, manages your `~/.ssh/config` with a protected block it owns, and keeps the ssh-agent loaded — all behind a clean command-line interface.

```
$ sshelf whoami

  Active profile:   work
  Git identity:     Michael Schumacher <ajmal@company.com>
  SSH key:          ~/.sshelf/keys/work_ed25519  (created 47 days ago)
  Agent:            running  ·  PID 12345  ·  2 keys loaded
  Host aliases:     github-work, gitlab-work
```

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
  - [profile](#sshelf-profile)
  - [key](#sshelf-key)
  - [host](#sshelf-host)
  - [agent](#sshelf-agent)
  - [doctor](#sshelf-doctor)
  - [import](#sshelf-import)
  - [whoami](#sshelf-whoami)
  - [completion](#sshelf-completion)
- [Shell Integration](#shell-integration)
- [How sshelf stores data](#how-sshelf-stores-data)
- [Migrating an existing SSH setup](#migrating-an-existing-ssh-setup)
- [Key rotation](#key-rotation)
- [Jump hosts (bastion servers)](#jump-hosts-bastion-servers)
- [Doctor checks reference](#doctor-checks-reference)
- [Security](#security)
- [Building from source](#building-from-source)
- [Contributing](#contributing)

---

## Installation

### macOS / Linux — pre-built binary (recommended)

Download the latest release binary for your platform from the [Releases page](https://github.com/ajf1016/sshelf/releases):

```bash
# macOS (Apple Silicon)
curl -Lo sshelf https://github.com/ajf1016/sshelf/releases/latest/download/sshelf-darwin-arm64
chmod +x sshelf && sudo mv sshelf /usr/local/bin/

# macOS (Intel)
curl -Lo sshelf https://github.com/ajf1016/sshelf/releases/latest/download/sshelf-darwin-amd64
chmod +x sshelf && sudo mv sshelf /usr/local/bin/

# Linux (x86_64)
curl -Lo sshelf https://github.com/ajf1016/sshelf/releases/latest/download/sshelf-linux-amd64
chmod +x sshelf && sudo mv sshelf /usr/local/bin/

# Linux (ARM64)
curl -Lo sshelf https://github.com/ajf1016/sshelf/releases/latest/download/sshelf-linux-arm64
chmod +x sshelf && sudo mv sshelf /usr/local/bin/
```

### go install

If you have Go 1.23+ installed:

```bash
go install github.com/ajf1016/sshelf/cmd/sshelf@latest
```

### Homebrew (macOS / Linux)

```bash
brew install ajf1016/tap/sshelf
```

> **Note:** The Homebrew tap is coming soon. Use the binary install above in the meantime.

### Verify the installation

```bash
sshelf --version
# sshelf v0.1.0
```

**Prerequisites:** `ssh-keygen` and `ssh-agent` must be available on your `$PATH`. They ship with macOS and every major Linux distro by default.

---

## Quick Start

A brand-new setup from zero to a working multi-account Git SSH config in under 3 minutes.

### 1. Create your first profile

```bash
sshelf profile init
```

This opens an interactive wizard:

```
? Profile name: work
? Profile type:
  ❯ git (GitHub / GitLab / Bitbucket)
    server (SSH into remote machines)
    client (custom / other)

? Platform:
  ❯ GitHub
    GitLab
    Bitbucket
    Other

? Email: ajmal@company.com
? Username / git user: Michael Schumacher

? Key name: work_ed25519
? Key type:
  ❯ ed25519 (recommended)
    rsa 4096

Profile "work" created.

Public key (work_ed25519):
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... ajmal@company.com

Add the public key to your platform before using this profile.
```

Copy that public key and paste it into **GitHub → Settings → SSH keys** (or your platform of choice).

### 2. Switch to the profile

```bash
eval $(sshelf profile switch work)
```

This sets `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, and `GIT_COMMITTER_EMAIL` in your current shell, and rewrites the `~/.ssh/config` managed block to use the work key.

### 3. Check everything looks good

```bash
sshelf doctor
```

```
  CHECK           STATUS   MESSAGE
  config-dir      PASS     ~/.sshelf exists with correct permissions (0700)
  permissions     PASS     all managed keys have correct permissions
  agent           PASS     ssh-agent running · PID 12345
  active-profile  PASS     active profile: work
  key-age         PASS     all keys within rotation window
  orphaned-keys   PASS     no orphaned keys
```

### 4. Add a second profile (personal)

```bash
sshelf profile init
# follow the wizard with your personal account details

# switch back and forth:
eval $(sshelf profile switch personal)
eval $(sshelf profile switch work)
```

---

## Commands

### `sshelf profile`

Manage SSH identity profiles. A profile is a named combination of email, username, key pair, and platform.

#### `sshelf profile init`

Start the interactive wizard to create a new profile and generate its key pair.

```bash
sshelf profile init
```

#### `sshelf profile list`

Show all profiles. The active profile is marked with a green `●`.

```bash
sshelf profile list

    NAME       TYPE    PLATFORM   EMAIL                   KEY               AGE
  ● work       git     github     ajmal@company.com       work_ed25519      47d
    personal   git     github     ajmal@personal.com      personal_ed25519  3mo
    staging    server  —          —                       staging_rsa       1y
```

#### `sshelf profile switch <name>`

Activate a profile. Rewrites the SSH config managed block and exports git identity env vars.

```bash
eval $(sshelf profile switch work)
```

> **Why `eval $(...)`?** A subprocess cannot export environment variables to its parent shell. `switch` prints `export GIT_AUTHOR_NAME=...` lines; wrapping with `eval` applies them to your current shell session. See [Shell Integration](#shell-integration) for an automatic wrapper.

#### `sshelf profile show <name>`

Print full details of a profile including its public key.

```bash
sshelf profile show work

  Profile:   work
  Type:      git
  Platform:  github
  Email:     ajmal@company.com
  Username:  Michael Schumacher
  Key:       work_ed25519
  Created:   2025-01-15

  Public key:
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... ajmal@company.com
```

#### `sshelf profile edit <name>`

Update profile fields interactively (email, username, key name, platform).

```bash
sshelf profile edit work
```

#### `sshelf profile remove <name>`

Delete a profile. Prompts for confirmation. Deletes the associated key pair unless `--keep-key` is passed.

```bash
sshelf profile remove old-client
sshelf profile remove old-client --keep-key   # keep the key files
sshelf profile remove old-client --yes        # skip confirmation
```

#### `sshelf profile clone <name> <new-name>`

Duplicate a profile as a starting point. The cloned profile shares the same key reference — useful for testing or temporary overrides before generating a new key.

```bash
sshelf profile clone work work-client-abc
```

#### `sshelf profile rename <old-name> <new-name>`

Rename a profile and update all host alias references automatically.

```bash
sshelf profile rename work work-github
```

---

### `sshelf key`

Manage SSH key pairs. Keys are stored under `~/.sshelf/keys/` with permissions enforced at `0600` (private) and `0644` (public).

#### `sshelf key generate`

Generate a new key pair without creating a full profile.

```bash
sshelf key generate --name staging_ed25519
sshelf key generate --name legacy_rsa --type rsa --comment "legacy server"
```

| Flag | Default | Description |
|---|---|---|
| `--name` | *(required)* | Base filename for the key pair |
| `--type` | `ed25519` | Key algorithm: `ed25519` or `rsa` |
| `--comment` | `""` | Comment embedded in the public key |

#### `sshelf key list`

List all managed keys with age and linked profile.

```bash
sshelf key list

  KEY                  TYPE      COMMENT               AGE    PROFILE
  work_ed25519         ed25519   ajmal@company.com     47d    work
  personal_ed25519     ed25519   ajmal@personal.com    3mo    personal
  staging_rsa          rsa       staging server        1y     —
```

#### `sshelf key add <path>`

Import an existing private key (and its `.pub` companion) into sshelf management. Permissions are fixed automatically.

```bash
sshelf key add ~/.ssh/id_ed25519
sshelf key add ~/Downloads/server.pem
```

#### `sshelf key remove <name>`

Delete a key pair. Prompts for confirmation.

```bash
sshelf key remove staging_rsa
sshelf key remove staging_rsa --yes   # skip confirmation
```

#### `sshelf key rotate <name>`

Generate a fresh key for an existing key name. The old key is archived as `<name>.bak` so you can remove it from platforms at your own pace before deleting it.

```bash
sshelf key rotate work_ed25519

  → Generating new ed25519 key…
  → Old key backed up to ~/.sshelf/keys/work_ed25519.bak

  New public key (add to GitHub/GitLab, then remove the old one):
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...new ajmal@company.com

  Once you've updated your platforms:
    sshelf key remove work_ed25519.bak
```

#### `sshelf key copy <name>`

Copy a key's public key to the clipboard. Falls back to printing if no clipboard tool is available.

```bash
sshelf key copy work_ed25519
# → Public key copied to clipboard.
```

---

### `sshelf host`

Manage SSH host aliases. Each alias maps a short name to a hostname, user, port, and key — sshelf writes these into your `~/.ssh/config` managed block so they work with any SSH tool.

#### `sshelf host add`

Add a host alias. If `--name` and `--hostname` are both provided, creates non-interactively; otherwise opens a prompt.

```bash
# Interactive:
sshelf host add

# Non-interactive:
sshelf host add --name staging --hostname staging.company.com --user ubuntu
sshelf host add --name prod --hostname 10.0.1.50 --user ec2-user --port 2222 --profile work
```

| Flag | Default | Description |
|---|---|---|
| `--name` | | Short alias (used in `ssh <alias>`) |
| `--hostname` | | Real hostname or IP |
| `--user` | | SSH username |
| `--port` | `22` | SSH port |
| `--key` | | Key name (overrides profile key) |
| `--profile` | | Link to a profile for key resolution |

#### `sshelf host list`

List all host aliases, grouped by profile.

```bash
sshelf host list

  Profile: work
    github-work       github.com               git      work_ed25519
    gitlab-work       gitlab.company.com       git      work_ed25519
    staging           staging.company.com      ubuntu   work_ed25519

  Profile: personal
    github-personal   github.com               git      personal_ed25519

  (unlinked)
    backup-vps        192.168.1.100            root     staging_rsa
```

Use `--profile <name>` to filter to one profile.

#### `sshelf host remove <alias>`

Remove a host alias and regenerate the SSH config block.

```bash
sshelf host remove staging
sshelf host remove staging --yes
```

#### `sshelf host test <alias>`

Run a quick connectivity check against the host alias using `ssh -T`. Useful after adding a new host or rotating a key.

```bash
sshelf host test github-work
# Hi ajf1016! You've successfully authenticated, but GitHub does not provide shell access.
```

#### `sshelf host connect <alias>`

Connect directly to a host alias. Equivalent to `ssh <alias>` but resolves through sshelf's managed config.

```bash
sshelf host connect staging
```

#### `sshelf host copy-id <alias>`

Push the linked profile's public key to a remote server using `ssh-copy-id`. Useful for provisioning a new server host.

```bash
sshelf host copy-id backup-vps
```

#### `sshelf host jump <alias>`

Set up a ProxyJump (bastion host) for an alias interactively, or supply `--via <bastion-alias>`.

```bash
sshelf host jump prod-app --via bastion
```

---

### `sshelf agent`

Manage the ssh-agent lifecycle. sshelf persists the agent's socket path and PID to `~/.sshelf/agent.env` so it survives across terminal sessions on the same login.

#### `sshelf agent start`

Start a new ssh-agent and load all managed keys.

```bash
eval $(sshelf agent start)

  ssh-agent started.
  PID: 12345
  export SSH_AUTH_SOCK=/tmp/ssh-xxx/agent.12345
  export SSH_AGENT_PID=12345
```

Pass `--load-keys=false` to start without loading keys.

#### `sshelf agent stop`

Terminate the running agent and clean up the env file.

```bash
sshelf agent stop
```

#### `sshelf agent status`

Show agent status, PID, socket path, and currently loaded keys.

```bash
sshelf agent status

  Agent:    running
  PID:      12345
  Socket:   /tmp/ssh-xxx/agent.12345

  Loaded keys:
    256 SHA256:abc... ajmal@company.com (ED25519)
    256 SHA256:xyz... ajmal@personal.com (ED25519)
```

#### `sshelf agent reload`

Stop the current agent and start a fresh one, reloading all keys from `~/.sshelf/keys/`. Useful after key rotation or profile switch.

```bash
sshelf agent reload
```

---

### `sshelf doctor`

Run a suite of health checks across your sshelf configuration and print a colour-coded results table.

```bash
sshelf doctor

  CHECK           STATUS   MESSAGE
  config-dir      PASS     ~/.sshelf exists with correct permissions (0700)
  permissions     WARN     work_ed25519 has permissions 0644, expected 0600  [fixable]
  agent           WARN     ssh-agent is not running
  active-profile  PASS     active profile: work
  key-age         WARN     staging_rsa is 14 months old — consider rotating
  orphaned-keys   WARN     old_rsa is not linked to any profile  [fixable]

  3 issue(s) found.  Run `sshelf doctor --fix` to auto-fix fixable issues.
```

#### Flags

| Flag | Description |
|---|---|
| `--fix` | Auto-apply all fixable issues, then re-run checks |
| `--check <name>` | Run a single named check |

```bash
sshelf doctor --fix
sshelf doctor --check permissions
sshelf doctor --check key-age
```

---

### `sshelf import`

Scan your existing `~/.ssh/config` and `~/.ssh/` key files and interactively migrate them into sshelf management. **Nothing is deleted or modified without your explicit selection.** Your SSH config entries outside the managed block are never touched.

```bash
sshelf import

  Found 3 key file(s) in ~/.ssh not yet managed by sshelf.

  ◉ id_ed25519
  ○ id_rsa_old
  ◉ work_rsa

  Found 4 host block(s) not yet managed by sshelf.

  ◉ github.com → github.com
  ◉ gitlab.company.com → gitlab.company.com
  ○ 192.168.1.5 → 192.168.1.5

  Link imported hosts to a profile?
  ❯ (none)
    work
    personal

  ✓ imported key: id_ed25519
  ✓ imported key: work_rsa
  ✓ imported host: github.com → github.com
  ✓ imported host: gitlab.company.com → gitlab.company.com

  4 item(s) imported successfully.
  SSH config updated. Keys stored in: ~/.sshelf/keys
```

After importing, run `sshelf doctor` to verify permissions and agent state.

---

### `sshelf whoami`

Print a snapshot of the current identity: active profile, git env vars, key age, agent state, and host aliases.

```bash
sshelf whoami

  Active profile:   work
  Git identity:     Michael Schumacher <ajmal@company.com>
  SSH key:          ~/.sshelf/keys/work_ed25519  (created 47 days ago)
  Agent:            running  ·  PID 12345  ·  2 keys loaded
  Host aliases:     github-work, gitlab-work, staging
```

---

### `sshelf completion`

Generate shell completion scripts. Tab-completes profile names, host aliases, and key names live from your store.

```bash
# Zsh
sshelf completion zsh >> ~/.zshrc && source ~/.zshrc

# Bash
sshelf completion bash >> ~/.bashrc && source ~/.bashrc

# Fish
sshelf completion fish > ~/.config/fish/completions/sshelf.fish
```

After sourcing, pressing `<Tab>` after a command resolves live names:

```bash
sshelf profile switch <Tab>
# work   personal   staging
```

---

## Shell Integration

`profile switch` outputs `export` statements that need to be `eval`-ed to take effect in your current shell. Add a thin wrapper to avoid typing `eval $(...)` every time:

**Zsh / Bash — add to `~/.zshrc` or `~/.bashrc`:**

```bash
sshelf-switch() {
  eval $(sshelf profile switch "$1")
}
```

```bash
sshelf-switch work
sshelf-switch personal
```

**Fish — add to `~/.config/fish/config.fish`:**

```fish
function sshelf-switch
  eval (sshelf profile switch $argv)
end
```

### Auto-start the agent on login

```bash
# ~/.zshrc or ~/.bashrc
if ! sshelf agent status &>/dev/null; then
  eval $(sshelf agent start)
fi
```

---

## How sshelf stores data

sshelf keeps all its state under `~/.sshelf/`. It never puts files directly in `~/.ssh/` — the only thing it touches there is a clearly delimited block inside `~/.ssh/config`.

| Path | Contents | Permissions |
|---|---|---|
| `~/.sshelf/` | Root config directory | `0700` |
| `~/.sshelf/profiles.toml` | All profile definitions | `0600` |
| `~/.sshelf/hosts.toml` | Host alias definitions | `0600` |
| `~/.sshelf/keys/` | Managed key pairs | `0700` |
| `~/.sshelf/keys/<name>` | Private key | `0600` |
| `~/.sshelf/keys/<name>.pub` | Public key | `0644` |
| `~/.sshelf/agent.env` | Agent PID + socket path (runtime) | `0600` |
| `~/.ssh/config` | Managed block only — rest is untouched | system |

### What the managed SSH config block looks like

```
# (your own host entries here — sshelf never touches these)

# BEGIN sshelf-managed — do not edit this block manually
Host github-work
  HostName github.com
  User git
  IdentityFile ~/.sshelf/keys/work_ed25519
  IdentitiesOnly yes

Host github-personal
  HostName github.com
  User git
  IdentityFile ~/.sshelf/keys/personal_ed25519
  IdentitiesOnly yes

Host staging
  HostName staging.company.com
  User ubuntu
  IdentityFile ~/.sshelf/keys/work_ed25519
# END sshelf-managed

# (more of your own entries here — also untouched)
```

`IdentitiesOnly yes` is written for every entry to prevent SSH from offering the wrong key to hosts where you manage multiple accounts (e.g., two GitHub profiles).

### `profiles.toml` format

```toml
[profiles.work]
type       = "git"
platform   = "github"
email      = "ajmal@company.com"
username   = "Michael Schumacher"
key_name   = "work_ed25519"
active     = true
created_at = "2025-01-15T10:30:00Z"

[profiles.personal]
type       = "git"
platform   = "github"
email      = "ajmal@personal.com"
username   = "ajmal"
key_name   = "personal_ed25519"
active     = false
created_at = "2025-01-10T08:00:00Z"
```

---

## Migrating an existing SSH setup

If you already have SSH keys and a `~/.ssh/config`, use `sshelf import` — it scans both and lets you pick what to bring in. Nothing is deleted from `~/.ssh/` until you explicitly tell sshelf to remove it.

**Typical migration path:**

```bash
# 1. Scan and import
sshelf import

# 2. Fix any permission or agent issues automatically
sshelf doctor --fix

# 3. Verify the first imported host works
sshelf host test github-work

# 4. Optionally remove hand-written entries in ~/.ssh/config that
#    sshelf now manages (the managed block takes precedence for any
#    alias it defines, so this step is safe to defer)
```

sshelf only rewrites the `# BEGIN sshelf-managed` … `# END sshelf-managed` block. Everything else in `~/.ssh/config` is left exactly as-is, always.

---

## Key rotation

SSH keys age. `sshelf key rotate` makes rotation painless:

```bash
sshelf key rotate work_ed25519
```

What happens step by step:

1. A new ed25519 key pair is generated in place.
2. The old private key is moved to `work_ed25519.bak` — not deleted, so you have time to remove it from platforms.
3. The SSH config block is updated to the new key immediately.
4. The new public key is printed for you to add to GitHub / GitLab / wherever.

Once you've added the new key to all platforms and revoked the old one:

```bash
sshelf key remove work_ed25519.bak
```

`sshelf doctor` will warn you when any key is older than 1 year.

---

## Jump hosts (bastion servers)

For accessing internal servers through a bastion:

```bash
# Add the bastion first
sshelf host add --name bastion --hostname bastion.company.com --user ajmal --profile work

# Add the internal server and link it through the bastion
sshelf host add --name prod-app --hostname 10.0.1.50 --user ubuntu --profile work
sshelf host jump prod-app --via bastion
```

Generated SSH config:

```
Host bastion
  HostName bastion.company.com
  User ajmal
  IdentityFile ~/.sshelf/keys/work_ed25519

Host prod-app
  HostName 10.0.1.50
  User ubuntu
  IdentityFile ~/.sshelf/keys/work_ed25519
  ProxyJump bastion
```

Connect through the bastion in one step:

```bash
ssh prod-app
# or:
sshelf host connect prod-app
```

---

## Doctor checks reference

`sshelf doctor` runs these six checks in order:

| Check | ID | Severity | Auto-fixable | What it checks |
|---|---|---|---|---|
| Config directory | `config-dir` | warn | yes | `~/.sshelf/` exists with `0700` permissions |
| Key permissions | `permissions` | fail | yes | Every private key is `0600`, public keys `0644` |
| Agent running | `agent` | warn | no | ssh-agent process is alive |
| Active profile | `active-profile` | warn | no | At least one profile is marked active |
| Key age | `key-age` | warn | no | No managed key is older than 1 year |
| Orphaned keys | `orphaned-keys` | warn | yes | No key file exists without a profile referencing it |

Run a single check by name:

```bash
sshelf doctor --check permissions
sshelf doctor --check key-age
```

---

## Security

- **Private keys** are stored at `0600` and `~/.sshelf/` at `0700`. `sshelf doctor` flags any drift from these permissions and can fix them automatically.
- **No network calls.** sshelf never transmits your keys or config to any remote service.
- **Atomic writes.** All config and key file writes use a temp-file-then-`os.Rename` pattern. A crash mid-write cannot produce a corrupt file.
- **`IdentitiesOnly yes`** is set on every generated SSH config entry, preventing SSH from offering a key to the wrong host when you have multiple accounts on the same service.
- **Key generation** delegates to the system's `ssh-keygen` binary. sshelf does not implement cryptography.
- **Confirmation required** for all destructive operations (`remove`, `rotate`). Pass `--yes` only in scripts.
- **sshelf never reads private key content.** It manages file paths, metadata, and permissions only.

---

## Building from source

Requires **Go 1.23+** and the system `ssh-keygen` / `ssh-agent` binaries.

```bash
git clone https://github.com/ajf1016/sshelf.git
cd sshelf

# Run tests
go test -race ./...

# Build (development)
go build -o sshelf ./cmd/sshelf
sudo mv sshelf /usr/local/bin/

# Build (release — strip debug symbols, inject version)
go build \
  -trimpath \
  -ldflags="-s -w -X github.com/ajf1016/sshelf/internal/config.AppVersion=v0.1.0" \
  -o sshelf \
  ./cmd/sshelf
```

### Cross-compile

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -o dist/sshelf-linux-amd64 ./cmd/sshelf

# macOS arm64
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -o dist/sshelf-darwin-arm64 ./cmd/sshelf
```

`CGO_ENABLED=0` is required on macOS with Go 1.23+ due to Darwin system framework linking.

---

## Contributing

Contributions are welcome. Before opening a PR:

1. **Run the test suite** with the race detector:
   ```bash
   go test -race ./...
   ```

2. **Run the linter** (golangci-lint is used in CI):
   ```bash
   go vet ./...
   golangci-lint run   # https://golangci-lint.run/usage/install/
   ```

3. **Keep commands thin.** Business logic belongs in `internal/core/`, not in `internal/commands/`. Commands parse args, call core, and format output — nothing more.

4. **Tests for core logic are required.** Command files don't need unit tests; packages under `internal/core/` do.

5. **No real credentials in tests.** Use `t.TempDir()` for all paths; never commit hostnames, emails, or key material.

### Project layout

```
sshelf/
├── cmd/sshelf/         # binary entry point — main.go only
├── internal/
│   ├── commands/       # cobra command wrappers — thin, no business logic
│   ├── config/         # constants, permission bits, default paths
│   ├── core/           # all business logic: profiles, keys, hosts, agent, doctor, importer
│   ├── platform/       # OS-specific helpers (clipboard)
│   ├── ui/             # lipgloss styles, table renderer, formatting helpers
│   ├── utils/          # atomic file I/O, typed errors
│   └── wizard/         # interactive huh forms for init and import flows
└── tests/              # integration tests
```

---

## License

MIT — see [LICENSE](LICENSE).

---

*Built with [cobra](https://github.com/spf13/cobra), [huh](https://github.com/charmbracelet/huh), [lipgloss](https://github.com/charmbracelet/lipgloss), and [BurntSushi/toml](https://github.com/BurntSushi/toml).*
