# ttyrant

A terminal dashboard that shows all your tmux sessions, with live status for any running Claude Code sessions. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- Lists all tmux sessions with their working directories
- Shows live Claude Code status via hooks: **working**, **needs input**, **done**, **starting**
- Audio notification when Claude finishes a task (after 15s idle)
- Attach to tmux sessions directly from the dashboard
- Kill tmux sessions with confirmation
- Linux and macOS support

## Install

```sh
go install github.com/pfnilsson/ttyrant@latest
```

Or build from source:

```sh
go build -o ttyrant .
```

## Setup

Install the Claude Code hooks for rich status:

```sh
ttyrant install-hooks
```

Verify everything works:

```sh
ttyrant doctor
```

## Usage

Launch the dashboard:

```sh
ttyrant
```

### Keybindings

| Key   | Action                          |
|-------|---------------------------------|
| `q`   | Quit                            |
| `j/k` | Navigate up/down                |
| `a`   | Attach to tmux session window 1 |
| `A`   | Attach to tmux session window 2 |
| `d`   | Kill tmux session (confirms)    |

### CLI Commands

```sh
ttyrant                  # Launch TUI dashboard
ttyrant scan --json      # List Claude Code processes as JSON
ttyrant install-hooks    # Install hooks into Claude Code settings
ttyrant uninstall-hooks  # Remove hooks from Claude Code settings
ttyrant doctor           # Run diagnostic checks
```

## How It Works

1. **tmux sessions** are listed as the primary rows
2. A **process scanner** discovers running Claude Code processes by inspecting system processes
3. **Claude Code hooks** fire lifecycle events (`SessionStart`, `PreToolUse`, `PermissionRequest`, `TaskCompleted`, etc.) that are captured by `ttyrant hook` and written to state files
4. The **merge layer** matches Claude data to tmux sessions by working directory
5. Sessions without Claude just show as `no claude`

See [docs/architecture.md](docs/architecture.md) for details.
