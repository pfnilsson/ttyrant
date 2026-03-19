# Troubleshooting

## Run doctor first

```sh
ttyrant doctor
```

This checks all components and reports what's working and what isn't.

## Common Issues

### "Hooks not installed" warning

Run `ttyrant install-hooks`. If it fails, check that `~/.claude/settings.json` exists and is valid JSON.

### ttyrant not found

The `ttyrant` binary must be in your `$PATH`. Check with:

```sh
which ttyrant
```

### No sessions showing

ttyrant shows tmux sessions. If you're not running tmux, there's nothing to display. Start a tmux session first.

### Claude status stuck on "unknown"

This means hooks aren't firing or the state files aren't being written. Check:

1. Hooks are installed: `ttyrant doctor`
2. State directory is writable: `ls -la ~/.local/state/ttyrant/`
3. Events are being logged: `tail -f ~/.local/state/ttyrant/events/$(date +%Y-%m-%d).log`

### Claude status stuck on old state

If hooks are working but status seems wrong, check the event log to see what's actually being received:

```sh
tail -20 ~/.local/state/ttyrant/events/$(date +%Y-%m-%d).log | python3 -m json.tool
```

### No sound playing

ttyrant needs one of these audio players: `paplay` (PulseAudio/PipeWire), `aplay` (ALSA), or `afplay` (macOS). Check:

```sh
which paplay aplay afplay
```

Sound only plays on `working → done` or `working → needs_input` transitions, and only if it's been at least 15 seconds since the user last submitted a prompt.

### State files piling up

State files for sessions without a live Claude process are cleaned up after 15 minutes. If stale files persist, you can safely delete the contents of `~/.local/state/ttyrant/current/`.
