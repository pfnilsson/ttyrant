# Architecture

## Overview

ttyrant is a single binary with multiple subcommands:

- **`ttyrant`** — TUI dashboard (default, no arguments)
- **`ttyrant hook`** — subcommand invoked by Claude Code hooks
- **`ttyrant scan`** — lists running Claude Code processes
- **`ttyrant install-hooks`** / **`uninstall-hooks`** — manage Claude Code hook registration
- **`ttyrant doctor`** — diagnostic checks

## Data Flow

```
Claude Code hooks ──► ttyrant hook ──► State files (~/.local/state/ttyrant/)
                                                │
tmux list-sessions ──► tmux.ListSessions()      │ read
                              │                  │
ps / /proc ──► scanner.Scan() │                  │
                    │         │                  │
                    └─────────┴──────────────────┘
                              │
                        merge.Merge()
                              │
                        Bubble Tea TUI
```

## Packages

### `internal/model`
Core data types: `SessionStatus`, `LiveProcess`, `HookState`, `SessionRow`.

### `internal/scanner`
Discovers Claude Code processes via `ps`. Platform-specific cwd resolution:
- Linux: reads `/proc/<pid>/cwd` symlink
- macOS: uses `lsof -p <pid>`

### `internal/hooks`
- **mapping.go** — maps Claude Code hook event names to `SessionStatus`
- **sink.go** — processes hook payloads from stdin, writes state files, appends event logs

### `internal/state`
Reads and writes per-directory JSON state files in `~/.local/state/ttyrant/current/`. Uses atomic writes (temp file + rename). State files are keyed by a SHA256 hash of the normalized directory path.

### `internal/tmux`
Lists tmux sessions, finds sessions by directory, generates attach/switch commands.

### `internal/merge`
Combines tmux sessions (primary rows), Claude processes, and hook state into `SessionRow` entries. Tmux sessions are always shown. Claude data enriches matching sessions by directory (exact match or child path).

### `internal/tui`
Bubble Tea model with 2-second refresh loop and two views:
- **Sessions view** — tmux sessions with Claude status, directory, last event, and idle time
- **Worktrees view** — git worktrees grouped by bare repo, with branch, head commit, and session status

### `internal/worktree`
Scans for bare git repositories under `~/Projects` and `~/.config`. Lists worktrees per repo with branch and head commit. Supports creating new worktrees (with tmux session), deleting worktrees (cleans up tmux session and git worktree), and cloning bare repos by URL.

### `internal/audio`
Embeds a notification sound via `//go:embed`. Plays asynchronously using the first available player (`paplay`, `aplay`, `afplay`). Only triggers on `working → done` or `working → needs_input` transitions when the user hasn't submitted a prompt in the last 15 seconds.

### `internal/install`
Manages hook registration in `~/.claude/settings.json`. Idempotent install/uninstall.

### `internal/doctor`
Runs diagnostic checks: scanner, state directory, hooks config, ttyrant in PATH.

## Design Decisions

1. **Tmux sessions as primary rows.** The dashboard shows all tmux sessions. Claude status is enrichment, not the primary view.

2. **No Claude = `no claude` status.** Sessions without a live Claude process always show as plain tmux sessions, regardless of stale hook data.

3. **Hook state is authoritative when fresh.** If a Claude process is alive and hook state exists and isn't stale (>5 min), trust it. Otherwise fall back to heuristics.

4. **Atomic state writes.** State files use write-to-temp + rename to prevent corruption from concurrent hook events.

5. **Sound cooldown.** Audio notifications are suppressed for 15 seconds after the user's last prompt to avoid noise during synchronous interaction.

6. **Directory matching.** Claude processes are matched to tmux sessions by cwd. Child directories also match (e.g., Claude running in `/project/subdir` matches tmux session at `/project`).

7. **Bare repo convention.** Worktree management assumes bare repos (`git clone --bare`). Worktrees are created as subdirectories of the bare repo and get their own tmux sessions with nvim + terminal windows.

8. **State caching.** Session rows are cached to `~/.local/state/ttyrant/cache.json` for fast startup.
