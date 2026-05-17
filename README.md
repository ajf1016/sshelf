# sshelf

> One CLI to manage all your SSH identities, keys, and server connections — no more hand-editing `~/.ssh/config`.

[![CI](https://github.com/ajf1016/sshelf/actions/workflows/ci.yml/badge.svg)](https://github.com/ajf1016/sshelf/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](#installation)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

---

## The problem

You are a developer on a single machine. You have:

- A **personal GitHub** account for side projects
- A **work GitHub** account that your company owns
- A **test server** your team gave you access to via a `.pem` key
- Maybe a **freelance client** with their own GitLab

Every time you sit down to work you're either:

- Googling *"how to use two GitHub accounts SSH"* again
- Digging through `~/Downloads` for a `.pem` file you got in Slack three months ago
- Breaking your `~/.ssh/config` with a stray edit
- Forgetting which email address commits to which repo

**sshelf fixes all of this.** It manages SSH keys, host aliases, and agent sessions behind one clean CLI. You never touch `~/.ssh/config` by hand again.

---

## Table of Contents

- [Installation](#installation)
- [Real-world scenarios](#real-world-scenarios)
  - [Two GitHub accounts on one machine](#scenario-1-two-github-accounts-on-one-machine)
  - [Your team gave you a .pem key for a server](#scenario-2-your-team-gave-you-a-pem-key-for-a-server)
  - [New laptop — restore everything from backup](#scenario-3-new-laptop--restore-everything-from-backup)
  - [Already have SSH keys — migrate without losing anything](#scenario-4-already-have-ssh-keys--migrate-without-losing-anything)
- [Commands](#commands)
  - [profile](#sshelf-profile)
  - [key](#sshelf-key)
  - [host](#sshelf-host)
  - [agent](#sshelf-agent)
  - [doctor](#sshelf-doctor)
  - [import](#sshelf-import)
  - [whoami](#sshelf-whoami)
  - [completion](#sshelf-completion)
  - [shell](#sshelf-shell)
- [Shell integration](#shell-integration)
- [How sshelf stores data](#how-sshelf-stores-data)
- [Key rotation](#key-rotation)
- [Jump hosts (bastion servers)](#jump-hosts-bastion-servers)
- [Doctor checks reference](#doctor-checks-reference)
- [Security](#security)
- [Building from source](#building-from-source)
- [Contributing](#contributing)

---

## Installation

### macOS / Linux — pre-built binary

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
```

### go install

```bash
go install github.com/ajf1016/sshelf/cmd/sshelf@latest
```

### Homebrew

```bash
brew install ajf1016/tap/sshelf
```

> The Homebrew tap is coming soon. Use the binary install above in the meantime.

**Prerequisites:** `ssh-keygen` and `ssh-agent` ship with macOS and every major Linux distro by default.

---

## Real-world scenarios

### Scenario 1: Two GitHub accounts on one machine

You have `alex@gmail.com` on personal GitHub and `alex@acme.io` on work GitHub. Both repos live on the same laptop.

**One-time setup:**

```bash
# Create your personal identity — generates a key pair
sshelf profile init
# Profile name: personal
# Type: git → GitHub
# Email: alex@gmail.com
# Username: alex-personal

# Create your work identity — generates a separate key pair
sshelf profile init
# Profile name: work
# Type: git → GitHub
# Email: alex@acme.io
# Username: alex-acme
```

sshelf generates two separate key pairs and writes this automatically to `~/.ssh/config`:

```
Host github-personal
  HostName github.com
  User git
  IdentityFile ~/.sshelf/keys/personal_ed25519
  IdentitiesOnly yes

Host github-work
  HostName github.com
  User git
  IdentityFile ~/.sshelf/keys/work_ed25519
  IdentitiesOnly yes
```

**Copy the public keys to GitHub** (you only do this once):

```bash
sshelf key copy personal_ed25519   # copies to clipboard → paste into GitHub personal settings
sshelf key copy work_ed25519       # copies to clipboard → paste into GitHub work settings
```

**Day-to-day use:**

```bash
# Clone a personal repo — use the alias, not github.com directly
git clone git@github-personal:alex/my-side-project.git

# Clone a work repo
git clone git@github-work:acme/backend-service.git

# Switch your active git identity in the current terminal
sshelf-switch work   # requires one-time: sshelf shell setup

# Verify who you are right now
sshelf whoami
#   Active profile:  work
#   Git identity:    alex <alex@acme.io>
#   SSH key:         ~/.sshelf/keys/work_ed25519  (created 12 days ago)
#   Agent:           running · PID 34521 · 2 keys loaded
#   Host aliases:    github-work
```

---

### Scenario 2: Your team gave you a .pem key for a server

Your senior Slacked you:

> "Here's the staging server key 👇  
> `ssh -i ~/Downloads/staging.pem ubuntu@203.0.113.42`"

You don't want to type that full command every time. You don't want that `.pem` rotting in `~/Downloads` forever.

**One-time setup:**

```bash
# Step 1 — bring the key into sshelf management
sshelf key add ~/Downloads/staging.pem
# → Imported key: staging (ed25519)
# → Key stored at ~/.sshelf/keys/staging.pem
# → Permissions set to 0600

# Step 2 — create a short alias for the server
sshelf host add
#   Host alias       → staging
#   Hostname or IP   → 203.0.113.42
#   SSH user         → ubuntu
#   Identity key     → staging.pem      ← tab-completes from your imported keys
```

**From now on:**

```bash
ssh staging
# or
sshelf host connect staging
```

Both do exactly the same thing as the original long command — but you never have to remember the IP, the username, or which `.pem` to use.

**Verify the connection works:**

```bash
sshelf host test staging
# → Testing connection to staging (203.0.113.42)...
# → Connected successfully as ubuntu
```

**Team grows — more servers:**

```bash
sshelf host add --name prod     --hostname 203.0.113.100 --user ubuntu --key staging.pem
sshelf host add --name dev-box  --hostname 203.0.113.55  --user ec2-user --key staging.pem

sshelf host list
#   ALIAS      HOSTNAME          USER       KEY
#   staging    203.0.113.42      ubuntu     staging.pem
#   prod       203.0.113.100     ubuntu     staging.pem
#   dev-box    203.0.113.55      ec2-user   staging.pem
```

---

### Scenario 3: New laptop — restore everything from backup

Before you wiped your old machine, you backed up all your sshelf keys:

```bash
sshelf key backup
# Passphrase: ••••••••••••
# Confirm:    ••••••••••••
# ✓ 6 key files backed up to ~/.sshelf/keys-backup-2026-05-17.vault
```

You copied that `.vault` file to a USB drive (or cloud storage). Now on the new machine:

```bash
# Install sshelf, then:
sshelf key restore /Volumes/USB/keys-backup-2026-05-17.vault
# Passphrase: ••••••••••••
# ✓ 6 key files restored to ~/.sshelf/keys/

sshelf doctor
# → All checks pass
```

Your profiles, host aliases, and every key are back exactly as they were. Nothing to reconfigure.

---

### Scenario 4: Already have SSH keys — migrate without losing anything

You already have `~/.ssh/id_ed25519`, `~/.ssh/work_rsa`, and a hand-crafted `~/.ssh/config`. You want sshelf to take over without losing anything.

```bash
sshelf import
```

sshelf scans your existing setup and lets you pick what to bring in:

```
→ Scanning ~/.ssh/config... found 3 Host blocks
→ Scanning ~/.ssh/... found 4 key files

  Select keys to import:
  ◉ id_ed25519
  ◉ work_rsa
  ○ id_rsa_old          ← left this one out

  Select hosts to import:
  ◉ github.com
  ◉ gitlab.acme.io
  ○ 192.168.0.5         ← skipped this one too

  ✓ Imported 2 keys, 2 hosts
  ✓ SSH config updated
```

Your hand-written entries outside the sshelf block are **never modified**. Safe to run at any time.

After importing:

```bash
sshelf doctor --fix   # fixes any permission issues on imported keys
```

---

## Commands

### `sshelf profile`

A **profile** is your identity: name, email, git username, and an SSH key pair. Create one per account you own.

---

#### `sshelf profile init`

Start the interactive wizard to create a new profile and generate its key pair.

```bash
sshelf profile init
```

Walks you through:

```
? Profile name:      work
? Profile type:      git (GitHub / GitLab / Bitbucket)
? Platform:          GitHub
? Email:             alex@acme.io
? Username:          alex-acme
? Key name:          work_ed25519      ← pre-filled, press Enter to accept
? Key type:          ed25519 (recommended)

✓ Profile "work" created!

Public key (paste into GitHub → Settings → SSH keys):
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alex@acme.io
```

---

#### `sshelf profile list`

Show all profiles. The active one is marked with `●`.

```bash
sshelf profile list

    NAME       TYPE    PLATFORM   EMAIL               KEY               AGE
  ● work       git     github     alex@acme.io        work_ed25519      12d
    personal   git     github     alex@gmail.com      personal_ed25519  3mo
    freelance  git     gitlab     alex@client.com     freelance_rsa     8mo
```

---

#### `sshelf profile switch <name>`

Activate a profile. Updates the SSH config block and exports git identity env vars to your shell.

After running [`sshelf shell setup`](#sshelf-shell) once, you use the short form:

```bash
sshelf-switch work
# Now git commits go out as alex@acme.io with the work key

sshelf-switch personal
# Now git commits go out as alex@gmail.com with the personal key
```

If shell integration is not installed, use the raw form:

```bash
eval $(sshelf profile switch work)
```

> **Why `eval`?** A subprocess cannot export env vars to its parent shell. `switch` prints `export GIT_AUTHOR_NAME=...` lines; `eval` applies them. `sshelf shell setup` installs a wrapper so you never type `eval $(...)` again.

---

#### `sshelf profile show <name>`

Print full details of a profile including its public key.

```bash
sshelf profile show work

  Profile:    work
  Type:       git
  Platform:   github
  Email:      alex@acme.io
  Username:   alex-acme
  Key:        work_ed25519
  Created:    2026-05-05

  Public key:
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alex@acme.io
```

Useful when you need to give your public key to a server admin or add it to a new platform.

---

#### `sshelf profile edit <name>`

Update profile fields interactively. Pre-filled with current values — press Enter to keep any field unchanged.

```bash
sshelf profile edit work
# Change email from alex@acme.io → alex@newcompany.io
```

---

#### `sshelf profile remove <name>`

Delete a profile and optionally its key pair.

```bash
sshelf profile remove freelance          # prompts for confirmation
sshelf profile remove freelance --yes    # skip confirmation
sshelf profile remove freelance --keep-key  # delete profile, keep key files
```

Also works as:

```bash
sshelf profile delete freelance
```

---

#### `sshelf profile clone <name> <new-name>`

Duplicate a profile as a starting point. Useful when onboarding to a new project that shares the same platform.

```bash
sshelf profile clone work work-clientx
# Now edit the clone to set the right email
sshelf profile edit work-clientx
```

---

#### `sshelf profile rename <old> <new>`

Rename a profile and update all host aliases that reference it automatically.

```bash
sshelf profile rename work acme-work
```

---

### `sshelf key`

Keys are stored under `~/.sshelf/keys/` with `0600` permissions enforced automatically.

---

#### `sshelf key generate`

Generate a standalone key pair without creating a full profile. Useful for server keys or temporary access.

```bash
sshelf key generate --name deploy_ed25519
sshelf key generate --name legacy_server --type rsa --comment "old server needs RSA"
```

| Flag | Default | Description |
|---|---|---|
| `--name` | *(required)* | Base filename for the key pair |
| `--type` | `ed25519` | `ed25519` or `rsa` |
| `--comment` | `<name>@sshelf` | Comment embedded in the public key |

---

#### `sshelf key list`

List all managed keys with age and which profile they belong to.

```bash
sshelf key list

  NAME                 TYPE      AGE    PROFILE
  work_ed25519         ed25519   12d    work
  personal_ed25519     ed25519   3mo    personal
  freelance_rsa        rsa       8mo    freelance
  staging.pem          rsa       47d    —           ← key without a profile (server key)
```

Keys with no profile are server keys imported via `key add`. They still work fine — they just aren't tied to a git identity.

---

#### `sshelf key add <path>`

Import an existing key into sshelf management. Copies it to `~/.sshelf/keys/` and sets correct permissions.

```bash
# Your senior gave you a server key
sshelf key add ~/Downloads/staging.pem

# Bring in a key you already had in ~/.ssh/
sshelf key add ~/.ssh/id_ed25519
```

After importing, the original file is no longer needed — sshelf owns a copy.

---

#### `sshelf key remove <name>`

Delete a managed key pair.

```bash
sshelf key remove old_rsa
sshelf key remove old_rsa --yes   # skip confirmation
```

**Note:** if a profile references this key, the profile's `key_name` field will be blank until you update it with `sshelf profile edit`.

---

#### `sshelf key rotate <name>`

Generate a new key pair for an existing name. The old key is archived as `<name>.bak` — not deleted — giving you time to update GitHub/GitLab before removing it.

```bash
sshelf key rotate work_ed25519

  → Generating new ed25519 key...
  → Old key backed up to ~/.sshelf/keys/work_ed25519.bak

  New public key (add this to GitHub, then revoke the old one):
  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...newkey alex@acme.io

  Remember to update your platforms, then run:
    sshelf key remove work_ed25519.bak
```

`sshelf doctor` will warn you when any key is older than 1 year.

---

#### `sshelf key copy <name>`

Copy a key's public key content to your clipboard. Falls back to printing if no clipboard tool is available.

```bash
sshelf key copy work_ed25519
# → Public key of "work_ed25519" copied to clipboard.
```

Paste directly into GitHub → Settings → SSH keys, or into a server's `authorized_keys`.

---

#### `sshelf key backup`

Encrypt all managed keys into a single vault file. You'll be prompted for a passphrase (min 8 characters).

```bash
sshelf key backup
# Passphrase: ••••••••••••
# Confirm:    ••••••••••••
# ✓ 4 key file(s) backed up to ~/.sshelf/keys-backup-2026-05-17.vault

# Custom output location — e.g. USB drive
sshelf key backup --out /Volumes/USB/my-ssh-keys.vault
```

The vault is AES-256-GCM encrypted. Without the passphrase it cannot be read. Store it on a USB drive, cloud storage, or your password manager.

---

#### `sshelf key restore <vault-file>`

Decrypt a vault file and restore keys into `~/.sshelf/keys/`. Existing files are skipped unless `--force` is passed.

```bash
sshelf key restore ~/.sshelf/keys-backup-2026-05-17.vault
# Passphrase: ••••••••••••
# ✓ 4 key file(s) restored to ~/.sshelf/keys/

# Overwrite existing keys (e.g. after key rotation gone wrong)
sshelf key restore ~/backup/my-ssh-keys.vault --force
```

After restoring, run `sshelf doctor` to verify permissions.

---

### `sshelf host`

A **host** is a saved SSH destination: an alias name, hostname or IP, login user, and which key to use. Once added, `ssh <alias>` works anywhere on your machine.

---

#### `sshelf host add`

Add a host alias interactively, or pass flags to skip the prompts.

```bash
# Interactive — shows autocomplete suggestions as you type:
sshelf host add
#   Host alias           → staging
#   Hostname or IP       → 203.0.113.42
#   SSH user             → ubuntu        ← suggests: ubuntu, root, ec2-user, admin...
#   Identity key name    → staging.pem   ← tab-completes from your imported keys
#   Link to profile      →               ← tab-completes from your profiles
#   ProxyJump / bastion  →

# Non-interactive (good for scripts):
sshelf host add --name staging --hostname 203.0.113.42 --user ubuntu --key staging.pem
sshelf host add --name prod    --hostname 203.0.113.100 --user ubuntu --profile work
```

| Flag | Description |
|---|---|
| `--name` | Short alias used in `ssh <alias>` |
| `--hostname` | Real hostname or IP address |
| `--user` | SSH login username |
| `--port` | SSH port (default 22) |
| `--key` | Key name to use (overrides profile key) |
| `--profile` | Link to a profile — inherits its key automatically |
| `--jump` | ProxyJump (bastion) alias |

---

#### `sshelf host list`

List all host aliases, grouped by profile.

```bash
sshelf host list

  Profile: work
    ALIAS           HOSTNAME            USER       KEY
    github-work     github.com          git        work_ed25519
    staging         203.0.113.42        ubuntu     work_ed25519
    prod            203.0.113.100       ubuntu     work_ed25519

  Profile: personal
    github-personal github.com          git        personal_ed25519

  (no profile)
    dev-box         203.0.113.55        ec2-user   staging.pem
```

Filter by profile:

```bash
sshelf host list --profile work
```

---

#### `sshelf host remove <alias>`

Remove a host alias and regenerate the SSH config block.

```bash
sshelf host remove old-staging
sshelf host remove old-staging --yes
```

---

#### `sshelf host test <alias>`

Run a quick connectivity check using `ssh -T`. Shows whether the connection and key are working.

```bash
sshelf host test github-work
# Hi alex-acme! You've successfully authenticated, but GitHub does not provide shell access.

sshelf host test staging
# Connected to 203.0.113.42 (OpenSSH_9.0)

sshelf host test broken-server
# ssh: connect to host 203.0.113.99 port 22: Connection refused
```

Run this after adding a new host or rotating a key to confirm everything still works.

---

#### `sshelf host connect <alias>`

Open a live SSH session to a host alias. Identical to `ssh <alias>` but explicitly goes through sshelf's config resolution.

```bash
sshelf host connect staging
# → opens a terminal session on the server

sshelf host connect prod
```

---

#### `sshelf host copy-id <alias>`

Push the linked profile's public key to a remote server's `authorized_keys` using `ssh-copy-id`. Use this to provision a new server before you have key-based auth set up — you'll be prompted for the password once.

```bash
sshelf host copy-id new-server
# → /usr/bin/ssh-copy-id: INFO: 1 key(s) remain to be installed.
# → alex@203.0.113.88's password: ••••••••
# → ✓ Key installed. You can now ssh in without a password.
```

---

#### `sshelf host jump <alias>`

Set or update a ProxyJump (bastion) for an alias — for reaching servers that aren't directly accessible from the internet.

```bash
sshelf host jump prod-app --via bastion
# → Set ProxyJump for "prod-app" → bastion
```

See [Jump hosts](#jump-hosts-bastion-servers) for the full bastion server workflow.

---

### `sshelf agent`

Manages the ssh-agent lifecycle. sshelf stores the agent's socket path and PID so keys stay loaded across terminal sessions on the same login.

---

#### `sshelf agent start`

Start a new ssh-agent and load all managed keys.

```bash
eval $(sshelf agent start)
# ssh-agent started.
# PID: 34521
```

#### `sshelf agent stop`

Kill the running agent.

```bash
sshelf agent stop
```

#### `sshelf agent status`

Show agent state, PID, and which keys are loaded.

```bash
sshelf agent status

  Agent:    running
  PID:      34521
  Socket:   /tmp/ssh-XxYyZz/agent.34521

  Loaded keys:
    256 SHA256:abc... alex@acme.io (ED25519)
    256 SHA256:xyz... alex@gmail.com (ED25519)
```

#### `sshelf agent reload`

Restart the agent and reload all keys. Run this after a profile switch or key rotation.

```bash
sshelf agent reload
```

---

### `sshelf doctor`

Run a suite of health checks and print a colour-coded table of results.

```bash
sshelf doctor

  CHECK           STATUS   MESSAGE
  config-dir      PASS     ~/.sshelf exists (0700)
  permissions     WARN     staging.pem has permissions 0644, expected 0600  [fixable]
  agent           WARN     ssh-agent is not running  [fixable]
  active-profile  PASS     active profile: work
  key-age         WARN     freelance_rsa is 14 months old — run: sshelf key rotate freelance_rsa
  orphaned-keys   PASS     no orphaned keys

  3 issue(s). Run `sshelf doctor --fix` to auto-fix fixable items.
```

Auto-fix everything fixable in one shot:

```bash
sshelf doctor --fix
```

Run a single check by name:

```bash
sshelf doctor --check permissions
sshelf doctor --check key-age
sshelf doctor --check agent
```

**Fixable checks:** `config-dir`, `permissions`, `orphaned-keys`
**Informational only:** `key-age`, `active-profile`, `agent` (agent restart requires `eval`)

---

### `sshelf import`

Already have SSH keys and a `~/.ssh/config`? This command scans your existing setup and lets you migrate everything into sshelf without losing anything. Entries outside the managed block are **never touched**.

```bash
sshelf import

  → Scanning ~/.ssh/config... found 3 Host blocks
  → Scanning ~/.ssh/... found 4 key files

  Select keys to import:
  ◉ id_ed25519          ← will be managed by sshelf
  ◉ work_rsa
  ○ id_rsa_old          ← leaving this one out

  Select hosts to import:
  ◉ github.com
  ◉ gitlab.acme.io
  ○ 192.168.0.5

  Link imported items to a profile?
  ❯ work

  ✓ Imported key: id_ed25519
  ✓ Imported key: work_rsa
  ✓ Imported host: github.com
  ✓ Imported host: gitlab.acme.io

  4 item(s) imported. SSH config updated. Keys stored in ~/.sshelf/keys/.
  Run `sshelf doctor` to verify everything looks good.
```

---

### `sshelf whoami`

Print a snapshot of your current identity at a glance. Useful at the start of a work session.

```bash
sshelf whoami

  Active profile:   work
  Git identity:     alex <alex@acme.io>
  SSH key:          ~/.sshelf/keys/work_ed25519  (created 12 days ago)
  Agent:            running · PID 34521 · 2 keys loaded
  Host aliases:     github-work, staging, prod
```

---

### `sshelf completion`

Generate shell completion scripts. After sourcing, `<Tab>` completes profile names, host aliases, and key names live from your store.

```bash
# Zsh
sshelf completion zsh >> ~/.zshrc && source ~/.zshrc

# Bash
sshelf completion bash >> ~/.bashrc && source ~/.bashrc

# Fish
sshelf completion fish > ~/.config/fish/completions/sshelf.fish
```

```bash
sshelf profile switch <Tab>
# work    personal    freelance

sshelf host connect <Tab>
# staging    prod    dev-box    github-work

sshelf key rotate <Tab>
# work_ed25519    personal_ed25519    freelance_rsa
```

---

### `sshelf shell`

Shell integration utilities.

#### `sshelf shell setup`

Install the `sshelf-switch` function into your shell config file. Run once after installing sshelf — then switch profiles without ever typing `eval $(...)` again.

```bash
sshelf shell setup
# ✓ Shell integration installed in ~/.zshrc
# Apply now:  source ~/.zshrc
# Then use:   sshelf-switch work
```

Detects your shell automatically (`$SHELL`). Supports zsh, bash, and fish. Safe to re-run — checks for existing installation and skips if already done.

---

## Shell integration

### `sshelf shell setup`

Run this **once** after installing sshelf. It detects your shell (zsh, bash, or fish) and appends the `sshelf-switch` function to your config file automatically.

```bash
sshelf shell setup
# ✓ Shell integration installed in ~/.zshrc
#
# Apply now (or open a new terminal):
#   source ~/.zshrc
#
# Then switch profiles with:
#   sshelf-switch work
#   sshelf-switch personal
```

From then on, switching is a single short command:

```bash
sshelf-switch work       # activates work profile + sets git identity
sshelf-switch personal   # activates personal profile
```

**What it installs** (you can also add this manually if you prefer):

Zsh / Bash (`~/.zshrc` or `~/.bashrc`):
```bash
# sshelf shell integration
sshelf-switch() {
  eval $(sshelf profile switch "$1")
}
```

Fish (`~/.config/fish/config.fish`):
```fish
# sshelf shell integration
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

sshelf keeps all state under `~/.sshelf/`. The only thing it touches in `~/.ssh/` is a clearly delimited block inside `~/.ssh/config`.

| Path | Contents | Permissions |
|---|---|---|
| `~/.sshelf/` | Root config directory | `0700` |
| `~/.sshelf/profiles.toml` | All profile definitions | `0600` |
| `~/.sshelf/hosts.toml` | Host alias definitions | `0600` |
| `~/.sshelf/keys/` | Managed key pairs | `0700` |
| `~/.sshelf/keys/<name>` | Private key | `0600` |
| `~/.sshelf/keys/<name>.pub` | Public key | `0644` |
| `~/.sshelf/keys-backup-<date>.vault` | AES-256-GCM encrypted key backup | `0600` |
| `~/.sshelf/agent.env` | Agent PID + socket path (runtime) | `0600` |
| `~/.ssh/config` | sshelf block only — everything else untouched | system |

### The managed SSH config block

```
# (your own entries above — sshelf never touches these)

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
  HostName 203.0.113.42
  User ubuntu
  IdentityFile ~/.sshelf/keys/staging.pem
# END sshelf-managed

# (your own entries below — also untouched)
```

`IdentitiesOnly yes` is set on every profile-linked entry to prevent SSH from accidentally offering the wrong key when you have multiple accounts on the same service (e.g. two GitHub accounts).

---

## Key rotation

SSH keys age. `sshelf key rotate` makes rotation painless:

```bash
sshelf key rotate work_ed25519
```

What happens:

1. A fresh ed25519 key pair is generated at the same path.
2. The old key is moved to `work_ed25519.bak` — not deleted yet.
3. The SSH config block updates immediately to the new key.
4. The new public key is printed for you to add to GitHub/GitLab.

Once you've added the new key to all platforms:

```bash
sshelf key remove work_ed25519.bak
```

`sshelf doctor` warns you when any managed key is older than 1 year.

---

## Jump hosts (bastion servers)

Many companies route internal server access through a bastion host. sshelf handles this natively.

**Setup:**

```bash
# Add the bastion first
sshelf host add --name bastion --hostname bastion.acme.io --user alex --profile work

# Add the internal app server
sshelf host add --name prod-app --hostname 10.0.1.50 --user ubuntu --profile work

# Link prod-app through bastion
sshelf host jump prod-app --via bastion
```

sshelf generates this in `~/.ssh/config`:

```
Host bastion
  HostName bastion.acme.io
  User alex
  IdentityFile ~/.sshelf/keys/work_ed25519

Host prod-app
  HostName 10.0.1.50
  User ubuntu
  IdentityFile ~/.sshelf/keys/work_ed25519
  ProxyJump bastion
```

Connect through the bastion in one command:

```bash
ssh prod-app
# or
sshelf host connect prod-app
```

---

## Doctor checks reference

| Check | ID | Severity | Auto-fixable | What it checks |
|---|---|---|---|---|
| Config directory | `config-dir` | warn | yes | `~/.sshelf/` exists with `0700` permissions |
| Key permissions | `permissions` | fail | yes | Every private key is `0600`, public keys `0644` |
| Agent running | `agent` | warn | no | ssh-agent process is alive and socket reachable |
| Active profile | `active-profile` | warn | no | At least one profile is marked active |
| Key age | `key-age` | warn | no | No managed key older than 1 year |
| Orphaned keys | `orphaned-keys` | warn | yes | No key file exists without a profile referencing it |

---

## Security

- **Private keys** are stored at `0600`. `sshelf doctor` flags any drift and can fix it automatically.
- **No network calls.** sshelf never transmits your keys or config to any remote service.
- **Atomic writes.** Every config and key write uses temp-file → `os.Rename`. A crash mid-write cannot corrupt your config.
- **`IdentitiesOnly yes`** on every generated entry prevents SSH from offering the wrong key to hosts where you have multiple accounts.
- **Key generation** delegates to the system `ssh-keygen` binary. sshelf does not implement cryptography for key generation.
- **Vault encryption** uses AES-256-GCM with a key derived from 100,000 rounds of iterated SHA-256 + a random 16-byte salt. Pure stdlib — no external crypto dependencies.
- **Destructive operations require confirmation.** Pass `--yes` only in scripts where you control the input.
- **sshelf never reads private key content.** It manages paths, metadata, and permissions only.

---

## Building from source

Requires **Go 1.23+** and the system `ssh-keygen` / `ssh-agent` binaries.

```bash
git clone https://github.com/ajf1016/sshelf.git
cd sshelf

# Run tests with race detector
go test -race ./...

# Development build
go build -o sshelf ./cmd/sshelf
sudo mv sshelf /usr/local/bin/

# Release build — strip debug info, inject version
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

---

## Contributing

Contributions are welcome. Before opening a PR:

1. **Run the test suite** with the race detector:
   ```bash
   go test -race ./...
   ```

2. **Run the linter:**
   ```bash
   go vet ./...
   golangci-lint run
   ```

3. **Keep commands thin.** Business logic belongs in `internal/core/`, not `internal/commands/`. Commands parse flags, call core services, and format output — nothing more.

4. **Tests for core logic are required.** Packages under `internal/core/` need test coverage. Command files don't.

5. **No real credentials in tests.** Use `t.TempDir()` for all paths; never commit hostnames, IPs, emails, or key material.

### Project layout

```
sshelf/
├── cmd/sshelf/         # binary entry point — main.go only
├── internal/
│   ├── commands/       # cobra command wrappers — thin, no business logic
│   ├── config/         # constants, permission bits, default paths
│   ├── core/           # all business logic: profiles, keys, hosts, agent, vault, doctor
│   ├── platform/       # OS-specific helpers (clipboard)
│   ├── ui/             # lipgloss styles, table renderer, formatting helpers
│   ├── utils/          # atomic file I/O, typed errors
│   └── wizard/         # interactive huh forms for init and import flows
```

---

## License

MIT — see [LICENSE](LICENSE).

---

*Built with [cobra](https://github.com/spf13/cobra), [huh](https://github.com/charmbracelet/huh), [lipgloss](https://github.com/charmbracelet/lipgloss), and [BurntSushi/toml](https://github.com/BurntSushi/toml).*
