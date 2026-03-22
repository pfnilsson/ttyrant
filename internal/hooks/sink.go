package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pfnilsson/ttyrant/internal/audio"
	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/state"
)

// HookPayload represents the JSON payload received from Claude Code hooks on stdin.
// We only parse the fields we need — the rest is preserved as raw JSON for logging.
type HookPayload struct {
	SessionID        string          `json:"session_id"`
	Cwd              string          `json:"cwd"`
	HookEventName    string          `json:"hook_event_name"`
	ToolName         string          `json:"tool_name,omitempty"`
	NotificationType string          `json:"notification_type,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

// ProcessHookEvent reads a hook payload from the given reader, updates
// the current-state file, and appends to the daily event log.
// pid is the PID of the Claude Code process (passed via environment or argument).
func ProcessHookEvent(r io.Reader, pid int) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var payload HookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	payload.Raw = raw

	if payload.Cwd == "" {
		return fmt.Errorf("payload missing cwd")
	}
	if payload.HookEventName == "" {
		return fmt.Errorf("payload missing hook_event_name")
	}

	now := time.Now()
	status := MapEventToStatus(payload.HookEventName)

	// Notification events are informational — don't change status.
	// The real PermissionRequest and Elicitation events already
	// handle those transitions. Notification (especially idle_prompt)
	// would cause spurious NEEDS INPUT after Claude goes idle.
	if payload.HookEventName == "Notification" {
		return nil
	}

	waitingReason := WaitingReason(payload.HookEventName)

	// Load existing state to check sequence ordering and guard transitions.
	existing, _ := state.ReadStateFile(payload.Cwd)
	seq := int64(now.UnixMicro())
	if existing != nil && existing.Sequence >= seq {
		// Out-of-order event — skip.
		return nil
	}

	// Guard: don't let Notification events override done/exited status.
	// After a task completes, Claude Code fires idle_prompt notifications
	// which would incorrectly flip status to needs_input. Only explicit
	// new-work events (UserPromptSubmit, SessionStart, PreToolUse) should
	// clear done/exited.
	if existing != nil && (existing.Status == model.StatusDone || existing.Status == model.StatusExited) {
		switch payload.HookEventName {
		case "UserPromptSubmit", "SessionStart", "PreToolUse", "SessionEnd":
			// These legitimately start new work or end the session — allow.
		default:
			// Everything else (Notification, SubagentStop, etc.) — keep existing status.
			return nil
		}
	}

	// Track when the user last submitted a prompt. This is used for the
	// sound cooldown — we only suppress sound if the user JUST sent a new
	// prompt (meaning they're actively watching). Permission grants and
	// elicitation replies are quick interactions that don't indicate the
	// user is watching, so they don't reset the cooldown.
	var lastPromptAt time.Time
	switch {
	case payload.HookEventName == "UserPromptSubmit":
		lastPromptAt = now
	case existing != nil:
		lastPromptAt = existing.LastPromptAt
	}

	// Sound: needs_input is always genuine (permission/elicitation) — play
	// immediately. Stop→done is debounced because Stop sometimes fires
	// mid-task and gets overridden by PreToolUse within ~250ms.
	if existing != nil && existing.Status == model.StatusWorking &&
		!lastPromptAt.IsZero() && now.Sub(lastPromptAt) >= soundCooldown {
		if status == model.StatusNeedsInput {
			audio.Play()
		} else if status == model.StatusDone {
			scheduleDebouncedSound(payload.Cwd, seq)
		}
	}

	// Don't write DONE for Stop immediately — the debounced _notify
	// process will promote to DONE after confirming no follow-up tool use.
	// This prevents the TUI from briefly flashing DONE on mid-task Stops.
	writeStatus := status
	if payload.HookEventName == "Stop" {
		writeStatus = model.StatusWorking
	}

	hookState := &model.HookState{
		Cwd:           payload.Cwd,
		PID:           pid,
		SessionID:     payload.SessionID,
		Event:         payload.HookEventName,
		Status:        writeStatus,
		LastEventAt:   now,
		WaitingReason: waitingReason,
		ToolName:      payload.ToolName,
		Sequence:      seq,
		LastPromptAt:  lastPromptAt,
		UpdatedBy:     "ttyrant hook",
	}

	if err := state.WriteState(hookState); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	if err := appendEventLog(now, payload); err != nil {
		// Log append is best-effort — don't fail the whole operation.
		fmt.Fprintf(os.Stderr, "ttyrant hook: warning: event log append failed: %v\n", err)
	}

	return nil
}

// appendEventLog appends a minimally transformed event to the daily log file.
func appendEventLog(t time.Time, payload HookPayload) error {
	if err := state.EnsureDirs(); err != nil {
		return err
	}

	filename := t.Format("2006-01-02") + ".log"
	path := filepath.Join(state.EventsDir(), filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := map[string]any{
		"timestamp":  t.Format(time.RFC3339Nano),
		"session_id": payload.SessionID,
		"cwd":        payload.Cwd,
		"event":      payload.HookEventName,
	}
	if payload.ToolName != "" {
		entry["tool_name"] = payload.ToolName
	}
	if payload.NotificationType != "" {
		entry["notification_type"] = payload.NotificationType
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = f.Write(data)
	return err
}


const soundCooldown = 15 * time.Second
const soundDebounce = 500 * time.Millisecond

// scheduleDebouncedSound spawns a detached ttyrant process that waits briefly,
// then plays sound only if the state hasn't been overridden by a newer event.
func scheduleDebouncedSound(cwd string, seq int64) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "_notify", cwd, fmt.Sprintf("%d", seq))
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Detach from parent so the hook process can exit immediately.
	cmd.SysProcAttr = nil
	_ = cmd.Start()
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
}

// RunNotify is the implementation of the _notify subcommand.
// It waits briefly, then checks if the Stop was real (no newer events).
// If so, it promotes the status to DONE and plays sound.
func RunNotify(cwd string, expectedSeq int64) {
	time.Sleep(soundDebounce)

	s, err := state.ReadStateFile(cwd)
	if err != nil || s == nil {
		return
	}
	if s.Sequence != expectedSeq {
		// A newer event arrived — the Stop was mid-task.
		return
	}
	// Sequence matches — the Stop was real. Promote to DONE.
	s.Status = model.StatusDone
	s.Sequence = time.Now().UnixMicro()
	_ = state.WriteState(s)
	audio.PlaySync()
}

// GetPIDFromEnv tries to get the Claude Code PID from environment variables.
// Falls back to PPID (the hook is invoked as a child of Claude).
func GetPIDFromEnv() int {
	// Check explicit env var first.
	if s := os.Getenv("TTYRANT_CLAUDE_PID"); s != "" {
		if pid, err := strconv.Atoi(s); err == nil {
			return pid
		}
	}
	// The hook process is spawned by Claude Code, so PPID is the Claude process.
	return os.Getppid()
}
