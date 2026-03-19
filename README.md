# ttyrant

A terminal dashboard for managing tmux sessions and git worktrees, with live Claude Code status. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- Live Claude Code status via hooks: **working**, **needs input**, **done**, **ready**, **starting**, **exited**
- Audio notification when Claude finishes a task
- Git worktree management: create, delete, and navigate worktrees for bare repos
- Clone bare repositories by URL
- Open projects from `~/Projects` or `~/.config` with branch picker
- Attach to tmux session windows (nvim / terminal)
- Kill tmux sessions and delete worktrees with confirmation
- Linux and macOS support

## Install

```sh
go install github.com/pfnilsson/ttyrant@latest
```

## Setup

Install the Claude Code hooks for live status:

```sh
ttyrant install-hooks
```

Verify everything works:

```sh
ttyrant doctor
```

## Usage

```sh
ttyrant
```

### Sessions View

| Key     | Action                        |
|---------|-------------------------------|
| `j/k`  | Navigate up/down              |
| `1`-`9`| Attach to session by number   |
| `a`    | Attach to window 1 (nvim)     |
| `A`    | Attach to window 2 (terminal) |
| `o`    | Open a project                |
| `w`    | Switch to Worktrees view      |
| `d`    | Kill session (confirms)       |
| `q`    | Quit                          |

### Worktrees View

| Key     | Action                        |
|---------|-------------------------------|
| `j/k`  | Navigate up/down              |
| `1`-`9`| Attach to worktree by number  |
| `a`    | Attach to window 1 (nvim)     |
| `A`    | Attach to window 2 (terminal) |
| `n`    | Create new worktree           |
| `C`    | Clone a bare repo             |
| `o`    | Open a project                |
| `d`    | Delete worktree (confirms)    |
| `w`    | Back to Sessions view         |
| `q`    | Quit                          |

### CLI Commands

```sh
ttyrant                  # Launch TUI dashboard
ttyrant scan --json      # List Claude Code processes as JSON
ttyrant install-hooks    # Install hooks into Claude Code settings
ttyrant uninstall-hooks  # Remove hooks
ttyrant doctor           # Run diagnostic checks
```

## How It Works

1. **Claude Code hooks** fire lifecycle events that get written to state files in `~/.local/state/ttyrant/`
2. A **process scanner** discovers running Claude Code processes via `ps` and resolves their working directories
3. **tmux sessions** are listed as primary rows in the dashboard
4. The **merge layer** matches Claude data to tmux sessions by working directory
5. **Worktree scanning** discovers bare repos under `~/Projects` and lists their worktrees with branch and status info

See [docs/architecture.md](docs/architecture.md) for details.
