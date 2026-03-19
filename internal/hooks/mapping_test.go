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
		{"TaskCompleted", model.StatusDone},
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
