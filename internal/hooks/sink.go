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

	// Notification events are informational — don't change status.
	if payload.HookEventName == "Notification" {
		return nil
	}

	now := time.Now()
	seq := int64(now.UnixMicro())

	// Load existing state to check sequence ordering and compute transition.
	existing, _ := state.ReadStateFile(payload.Cwd)
	if existing != nil && existing.Sequence >= seq {
		return nil
	}

	// Determine next status via FSM transition table, or initial mapping
	// when this is the first event for this cwd.
	var status model.SessionStatus
	if existing != nil {
		next, allowed := Transition(existing.Status, payload.HookEventName)
		if !allowed {
			return nil
		}
		status = next
	} else {
		status = MapEventToStatus(payload.HookEventName)
	}

	waitingReason := WaitingReason(payload.HookEventName)

	// Track when the user last submitted a prompt. This is used for the
	// sound cooldown — we only suppress sound if the user JUST sent a new
	// prompt (meaning they're actively watching).
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
	soundAllowed := existing != nil && existing.Status == model.StatusWorking &&
		!lastPromptAt.IsZero() && now.Sub(lastPromptAt) >= soundCooldown
	if soundAllowed && status == model.StatusNeedsInput {
		audio.Play()
	}

	// Always schedule debounced promotion for Stop events — status
	// promotion must not be gated on sound cooldown. Sound is optional.
	if status == model.StatusDone && existing != nil && existing.Status == model.StatusWorking {
		playSound := soundAllowed
		scheduleDebouncedDone(payload.Cwd, seq, playSound)
	}

	// Don't write DONE for Stop immediately — the debounced _notify
	// process will promote to DONE after confirming no follow-up tool use.
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

// scheduleDebouncedDone spawns a detached ttyrant process that waits briefly,
// then promotes the status to DONE if no new work has started. Sound is only
// played when playSound is true (i.e. sound cooldown has elapsed).
func scheduleDebouncedDone(cwd string, seq int64, playSound bool) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	args := []string{"_notify", cwd, fmt.Sprintf("%d", seq)}
	if !playSound {
		args = append(args, "--no-sound")
	}
	cmd := exec.Command(exe, args...)
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
// If so, it promotes the status to DONE and optionally plays sound.
func RunNotify(cwd string, expectedSeq int64, playSound bool) {
	time.Sleep(soundDebounce)

	s, err := state.ReadStateFile(cwd)
	if err != nil || s == nil {
		return
	}
	if s.Sequence != expectedSeq {
		// A newer event arrived. Only abort if it represents actual new
		// work (e.g. PreToolUse). Completion events like SubagentStop or
		// PostToolUse are just cleanup from the task that ended — they
		// shouldn't cancel the Stop→Done promotion.
		if isNewWorkEvent(s.Event) {
			return
		}
	}
	// Sequence matches — the Stop was real. Promote to DONE.
	s.Status = model.StatusDone
	s.Sequence = time.Now().UnixMicro()
	_ = state.WriteState(s)
	if playSound {
		audio.PlaySync()
	}
}

// isNewWorkEvent returns true for events that indicate Claude started new work
// after a Stop, as opposed to cleanup from a completed task.
func isNewWorkEvent(event string) bool {
	switch event {
	case "PreToolUse", "UserPromptSubmit", "SubagentStart", "SessionStart", "ElicitationResult":
		return true
	}
	return false
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
