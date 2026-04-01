# aw — git worktree for polyrepos

Lightweight CLI that creates isolated workspaces across multiple repositories using git worktrees, with automatic AI context file linking and Claude session management.

![aw new workflow](docs/workflow.svg)

## Install

```bash
go install github.com/lldxflwb/aw@latest
```

Or download a binary from [Releases](https://github.com/lldxflwb/aw/releases).

## Usage

### `aw new` — Create a workspace

```bash
cd ~/projects

# Basic: create workspace with new branch
aw new -b feature/login --dir /tmp/feature-login

# Auto directory: omit --dir, creates ../projects-feature-login
aw new -b feature/login

# Update + clone session + new branch
aw new -usb feature/login

# Based on an existing branch (not HEAD)
aw new -b feature/v2 --from feature/v1

# Short form with all options
aw new -usb feature/login -f dev
```

This will:
1. Scan for all git repos in the current directory
2. Fetch and create a git worktree for each repo with the given branch
3. Symlink AI context files (CLAUDE.md, .claude/, etc.)
4. Optionally clone Claude session and symlink project memory
5. Register workspace in `.aw/registry.json`

#### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--dir <path>` | | Target workspace directory (default: `../<cwd>-<branch>`) |
| `-b <branch>` | | New branch name (required) |
| `--from <branch>` | `-f` | Base branch to create from (default: HEAD) |
| `--update` | `-u` | Fetch remotes before creating worktrees |
| `--clone-session` | `-s` | Clone latest Claude session to new workspace |
| `--session-limit N` | | Clone N most recent sessions (max 10) |
| `--session-id <uuid>` | | Clone a specific session |
| `--json` | | JSON output |

Short flags can be combined: `-usb feature/login` = update + clone session + branch.

### `aw list` — List all workspaces

```bash
aw list
```

```
DIR                         BRANCH         REPOS  CREATED   STATUS
--------------------------  -------------  -----  --------  ------
/tmp/feature-login          feature/login  7      2h ago    ok
/tmp/fix-bug                fix/bug-123    7      1d ago    ok
```

### `aw status` — Show workspace status

```bash
cd /tmp/feature-login
aw status
```

```
REPO      BRANCH         STATUS  AHEAD  BEHIND  LAST COMMIT
--------  -------------  ------  -----  ------  ---------------------------
backend   feature/login  2M      1      0       a1b2c3d Add auth endpoint
frontend  feature/login  clean   0      0       e4f5g6h Update login page
```

Options: `--short` for tab-separated, `--json` for machine-readable.

### `aw rm` — Remove a workspace

```bash
# Basic remove (checks for dirty repos)
aw rm

# Force remove + delete branches
aw rm -fb

# Save sessions back to source before removing
aw rm -fb --save-session
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Force remove even with dirty repos |
| `--branch` | `-b` | Delete branches after removing worktrees |
| `--save-session` | | Move workspace sessions back to source project |
| `--dry-run` | | Preview without executing |
| `--dir <path>` | | Explicit workspace directory |

With `--force`, the workspace directory is fully removed. Without it, remaining files are listed with a hint.

### `aw prune` — Clean up stale registry entries

```bash
aw prune          # Remove entries for deleted directories
aw prune --force  # Also remove invalid/mismatched entries
```

### `aw relink` — Fix Windows context links

Converts copy-based context links back to symlinks after enabling Developer Mode.

## Configuration (`aw.yml`)

Auto-created on first run in the source directory:

```yaml
# AI context files to symlink into workspaces
context:
  - CLAUDE.md
  - AGENTS.md
  - codex.md
  - .claude
  - .codex
  - .cursorrules
  - .cursor

# Per-repo checkout branch (overrides --from)
branches:
  backend: dev
  frontend: main
  proto: main
```

The `branches` map lets you configure different base branches per repo, so you don't need `--from` every time.

## Session Clone

When creating a workspace with `-s`, aw:
- Copies the most recent Claude session (with a new UUID) to the new workspace's project directory
- Symlinks the project memory directory for shared context
- Records cloned session IDs in `workspace.json`

On `aw rm --save-session`, new sessions created in the workspace are moved back to the source project directory.

## JSON Protocol

All `--json` output follows this envelope:

```json
{
  "schema_version": 1,
  "ok": true,
  "data": { ... },
  "error": null,
  "warnings": []
}
```

Exit codes: `0` success, `1` partial failure, `2` usage error.

## License

MIT
