# Hook Setup

## Automatic Installation

```sh
ttyrant install-hooks
```

This adds hook entries to `~/.claude/settings.json`. It merges with existing hooks — your other hooks are preserved.

To preview the config without installing:

```sh
ttyrant install-hooks --print
```

## What Gets Installed

ttyrant registers hooks for these Claude Code lifecycle events:

| Event | Status |
|---|---|
| `SessionStart` | starting |
| `UserPromptSubmit` | working |
| `PreToolUse` | working |
| `PostToolUse` | working |
| `PostToolUseFailure` | working |
| `SubagentStart` | working |
| `SubagentStop` | working |
| `ElicitationResult` | working |
| `PermissionRequest` | needs_input |
| `Elicitation` | needs_input |
| `TaskCompleted` | done |
| `Stop` | done |
| `SessionEnd` | exited |

Each hook calls `ttyrant hook` which reads the event payload from stdin and writes a state file.

## Requirements

- `ttyrant` must be in your `$PATH`
- Run `ttyrant doctor` to verify

## Uninstall

```sh
ttyrant uninstall-hooks
```

Removes only the ttyrant hook entries. Other hooks in your settings are left intact.

## State Files

Hook state is written to:

```
~/.local/state/ttyrant/current/<hash>.json   # current state per directory
~/.local/state/ttyrant/events/YYYY-MM-DD.log # daily event log
```

The current state files are what the TUI reads each refresh. Event logs are append-only and useful for debugging.

## Without Hooks

ttyrant works without hooks — it just can't show semantic status. Sessions with Claude running will show `unknown` instead of `working`/`done`/`needs_input`. A warning banner in the TUI reminds you to install hooks.
