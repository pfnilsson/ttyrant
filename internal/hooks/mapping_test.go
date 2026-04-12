package hooks

import (
	"testing"

	"github.com/pfnilsson/ttyrant/internal/model"
)

func TestMapEventToStatus(t *testing.T) {
	tests := []struct {
		event string
		want  model.SessionStatus
	}{
		{"SessionStart", model.StatusStarting},
		{"UserPromptSubmit", model.StatusWorking},
		{"PreToolUse", model.StatusWorking},
		{"PostToolUse", model.StatusWorking},
		{"PostToolUseFailure", model.StatusWorking},
		{"SubagentStart", model.StatusWorking},
		{"SubagentStop", model.StatusWorking},
		{"ElicitationResult", model.StatusWorking},
		{"PermissionRequest", model.StatusNeedsInput},
		{"Elicitation", model.StatusNeedsInput},
		{"TaskCompleted", model.StatusWorking},
		{"Stop", model.StatusDone},
		{"SessionEnd", model.StatusExited},
		{"SomethingUnknown", model.StatusUnknown},
		{"", model.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := MapEventToStatus(tt.event)
			if got != tt.want {
				t.Errorf("MapEventToStatus(%q) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    model.SessionStatus
		event   string
		want    model.SessionStatus
		allowed bool
	}{
		// Working transitions
		{"working+pretooluse", model.StatusWorking, "PreToolUse", model.StatusWorking, true},
		{"working+permission", model.StatusWorking, "PermissionRequest", model.StatusNeedsInput, true},
		{"working+stop", model.StatusWorking, "Stop", model.StatusDone, true},
		{"working+sessionend", model.StatusWorking, "SessionEnd", model.StatusExited, true},

		// NeedsInput guards — completion events rejected
		{"needsinput+posttooluse", model.StatusNeedsInput, "PostToolUse", model.StatusNeedsInput, false},
		{"needsinput+subagent_stop", model.StatusNeedsInput, "SubagentStop", model.StatusNeedsInput, false},
		{"needsinput+taskcomp", model.StatusNeedsInput, "TaskCompleted", model.StatusNeedsInput, false},
		{"needsinput+postfail", model.StatusNeedsInput, "PostToolUseFailure", model.StatusNeedsInput, false},
		// NeedsInput — new work clears it
		{"needsinput+pretooluse", model.StatusNeedsInput, "PreToolUse", model.StatusWorking, true},
		{"needsinput+prompt", model.StatusNeedsInput, "UserPromptSubmit", model.StatusWorking, true},
		{"needsinput+elicitation_result", model.StatusNeedsInput, "ElicitationResult", model.StatusWorking, true},
		// NeedsInput — same-kind events stay
		{"needsinput+permission", model.StatusNeedsInput, "PermissionRequest", model.StatusNeedsInput, true},

		// Done guards
		{"done+subagent_stop", model.StatusDone, "SubagentStop", model.StatusDone, false},
		{"done+posttooluse", model.StatusDone, "PostToolUse", model.StatusDone, false},
		{"done+prompt", model.StatusDone, "UserPromptSubmit", model.StatusWorking, true},
		{"done+pretooluse", model.StatusDone, "PreToolUse", model.StatusWorking, true},
		{"done+sessionstart", model.StatusDone, "SessionStart", model.StatusStarting, true},

		// Exited guards
		{"exited+subagent_stop", model.StatusExited, "SubagentStop", model.StatusExited, false},
		{"exited+prompt", model.StatusExited, "UserPromptSubmit", model.StatusWorking, true},

		// Unknown event
		{"working+unknown", model.StatusWorking, "SomethingNew", model.StatusWorking, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, allowed := Transition(tt.from, tt.event)
			if allowed != tt.allowed {
				t.Errorf("Transition(%q, %q) allowed = %v, want %v", tt.from, tt.event, allowed, tt.allowed)
			}
			if got != tt.want {
				t.Errorf("Transition(%q, %q) = %q, want %q", tt.from, tt.event, got, tt.want)
			}
		})
	}
}

func TestWaitingReason(t *testing.T) {
	tests := []struct {
		event string
		want  string
	}{
		{"PermissionRequest", "permission"},
		{"Elicitation", "elicitation"},
		{"PostToolUse", ""},
		{"SessionStart", ""},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := WaitingReason(tt.event)
			if got != tt.want {
				t.Errorf("WaitingReason(%q) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}
