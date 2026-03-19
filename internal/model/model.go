package model

import "time"

// SessionStatus represents the semantic status of a Claude Code session.
type SessionStatus string

const (
	StatusActive     SessionStatus = "active" // tmux session, no Claude
	StatusStarting   SessionStatus = "starting"
	StatusWorking    SessionStatus = "working"
	StatusNeedsInput SessionStatus = "needs_input"
	StatusReady      SessionStatus = "ready" // Claude finished, waiting for next prompt
	StatusDone       SessionStatus = "done"
	StatusExited     SessionStatus = "exited"
	StatusUnknown    SessionStatus = "unknown"
)

// StatusSource indicates where the status was derived from.
type StatusSource string

const (
	SourceHooks     StatusSource = "hooks"
	SourceHeuristic StatusSource = "heuristic"
	SourceUnknown   StatusSource = "unknown"
)

// LiveProcess represents a discovered Claude Code process.
type LiveProcess struct {
	PID        int
	PPID       int
	Cmdline    string
	Cwd        string
	TTY        string
	StartTime  time.Time
	LastSeenAt time.Time
	Transport  string // "pty", "tmux", "unknown"
}

// HookState represents the last known state from a Claude Code hook event.
type HookState struct {
	Cwd           string        `json:"cwd"`
	PID           int           `json:"pid"`
	SessionID     string        `json:"session_id"`
	Event         string        `json:"event"`
	Status        SessionStatus `json:"status"`
	LastEventAt   time.Time     `json:"last_event_at"`
	WaitingReason string        `json:"waiting_reason"`
	ToolName      string        `json:"tool_name"`
	Sequence      int64         `json:"sequence"`
	LastPromptAt  time.Time     `json:"last_prompt_at"`
	RawPayloadRef string        `json:"raw_payload_ref"`
	UpdatedBy     string        `json:"updated_by"`
}

// SessionRow is the merged, renderable row for the TUI.
type SessionRow struct {
	SessionName   string // tmux session name
	Cwd           string
	PID           int
	Status        SessionStatus
	StatusSource  StatusSource
	HasClaude     bool
	WaitingReason string
	LastEvent     string
	LastEventAt   time.Time
	Running       bool
	Transport     string
	Cmdline       string
	StartedAt     time.Time
	IdleFor       time.Duration
	LastPromptAt  time.Time
	Warning       string
}
