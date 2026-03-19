package state

import (
	"testing"
	"time"

	"github.com/pfnilsson/ttyrant/internal/model"
)

func TestReadStateFile_NotExist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	s, err := ReadStateFile("/nonexistent/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Error("expected nil for non-existent state file")
	}
}

func TestReadStateFile_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	want := &model.HookState{
		Cwd:           "/home/user/project",
		PID:           1234,
		SessionID:     "s1",
		Event:         "PreToolUse",
		Status:        model.StatusWorking,
		LastEventAt:   time.Now().Truncate(time.Millisecond),
		WaitingReason: "",
		ToolName:      "Bash",
		Sequence:      42,
		UpdatedBy:     "ttyrant hook",
	}

	if err := WriteState(want); err != nil {
		t.Fatal(err)
	}

	got, err := ReadStateFile("/home/user/project")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected state, got nil")
	}
	if got.Cwd != want.Cwd {
		t.Errorf("Cwd = %q, want %q", got.Cwd, want.Cwd)
	}
	if got.Status != want.Status {
		t.Errorf("Status = %q, want %q", got.Status, want.Status)
	}
}

func TestReadAllStates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	// Write two states.
	for _, cwd := range []string{"/project-a", "/project-b"} {
		s := &model.HookState{
			Cwd:    cwd,
			PID:    1234,
			Status: model.StatusWorking,
		}
		if err := WriteState(s); err != nil {
			t.Fatal(err)
		}
	}

	states, err := ReadAllStates()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("got %d states, want 2", len(states))
	}
}

func TestReadAllStates_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	states, err := ReadAllStates()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
}
