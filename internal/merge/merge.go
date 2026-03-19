package merge

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/state"
	"github.com/pfnilsson/ttyrant/internal/tmux"
)

const (
	// startingGracePeriod is how long a live process without hook state
	// is considered "starting" before falling back to "unknown".
	startingGracePeriod = 10 * time.Second

	// staleHookThreshold is how long since the last hook event before
	// we consider the hook state stale and degrade confidence.
	staleHookThreshold = 5 * time.Minute

	// retentionWindow is how long dead sessions (done/exited) are kept
	// visible before being removed.
	retentionWindow = 15 * time.Minute
)

// Merge combines tmux sessions, live Claude processes, and hook state data
// into a unified list of SessionRows. Tmux sessions are the primary rows;
// Claude data enriches matching sessions by directory.
func Merge(tmuxSessions []tmux.Session, processes []model.LiveProcess, hookStates []model.HookState, now time.Time) []model.SessionRow {
	// Index hook states by cwd for fast lookup.
	hookByCwd := make(map[string]*model.HookState, len(hookStates))
	for i := range hookStates {
		hookByCwd[hookStates[i].Cwd] = &hookStates[i]
	}

	// Index live processes by cwd. If multiple processes share a cwd,
	// keep the one with the highest PID (most recent).
	procByCwd := make(map[string]*model.LiveProcess, len(processes))
	for i := range processes {
		p := &processes[i]
		if existing, ok := procByCwd[p.Cwd]; !ok || p.PID > existing.PID {
			procByCwd[p.Cwd] = p
		}
	}

	var rows []model.SessionRow
	for _, sess := range tmuxSessions {
		proc, hook := matchClaude(sess.Path, procByCwd, hookByCwd)
		row := buildRow(sess, proc, hook, now)
		if row != nil {
			rows = append(rows, *row)
		}
	}

	return rows
}

// matchClaude finds a Claude process and hook state that match the tmux
// session path. Matches on exact cwd or cwd being a child of the session path.
func matchClaude(sessPath string, procByCwd map[string]*model.LiveProcess, hookByCwd map[string]*model.HookState) (*model.LiveProcess, *model.HookState) {
	sessPath = filepath.Clean(sessPath)

	// Try exact match first.
	proc := procByCwd[sessPath]
	hook := hookByCwd[sessPath]
	if proc != nil || hook != nil {
		return proc, hook
	}

	// Try child directories (Claude cwd is under the tmux session path).
	var bestProc *model.LiveProcess
	var bestHook *model.HookState
	for cwd, p := range procByCwd {
		if strings.HasPrefix(cwd, sessPath+"/") {
			bestProc = p
		}
	}
	for cwd, h := range hookByCwd {
		if strings.HasPrefix(cwd, sessPath+"/") {
			bestHook = h
		}
	}
	return bestProc, bestHook
}

// buildRow creates a SessionRow from a tmux session, optionally enriched
// with Claude process and hook data.
func buildRow(sess tmux.Session, proc *model.LiveProcess, hook *model.HookState, now time.Time) *model.SessionRow {
	row := &model.SessionRow{
		SessionName: sess.Name,
		Cwd:         sess.Path,
		Running:     true,
		Transport:   "tmux",
	}

	alive := proc != nil

	if !alive {
		// No live Claude process — show as plain tmux session.
		row.Status = model.StatusActive
		row.StatusSource = model.SourceUnknown
		// Clean up stale hook state if expired.
		if hook != nil && now.Sub(hook.LastEventAt) > retentionWindow {
			_ = state.RemoveState(hook.Cwd)
		}
		return row
	}

	row.HasClaude = true

	if alive {
		row.PID = proc.PID
		row.Cmdline = proc.Cmdline
		row.StartedAt = proc.StartTime
	}

	if hook != nil {
		row.LastEvent = hook.Event
		row.LastEventAt = hook.LastEventAt
		row.LastPromptAt = hook.LastPromptAt
		row.WaitingReason = hook.WaitingReason
		if row.PID == 0 {
			row.PID = hook.PID
		}
	}

	// Determine status and source.
	if hook != nil {
		stale := now.Sub(hook.LastEventAt) > staleHookThreshold
		if stale {
			// If Claude was starting, done, or waiting for input, it's ready
			// for the next prompt. Only truly ambiguous states degrade to unknown.
			switch hook.Status {
			case model.StatusStarting, model.StatusDone, model.StatusNeedsInput:
				row.Status = model.StatusReady
			default:
				row.Status = model.StatusUnknown
				row.Warning = "hook state stale"
			}
			row.StatusSource = model.SourceHeuristic
		} else {
			row.Status = hook.Status
			row.StatusSource = model.SourceHooks
		}
	} else {
		age := now.Sub(proc.StartTime)
		if age < startingGracePeriod {
			row.Status = model.StatusStarting
		} else {
			row.Status = model.StatusUnknown
		}
		row.StatusSource = model.SourceHeuristic
	}

	// Compute IdleFor: time since last hook event (if available).
	if hook != nil && !hook.LastEventAt.IsZero() {
		row.IdleFor = now.Sub(hook.LastEventAt)
	}

	return row
}
