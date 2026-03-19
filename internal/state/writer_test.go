package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pfnilsson/ttyrant/internal/model"
)

func TestWriteState_AtomicWrite(t *testing.T) {
	// Use a temp dir as state dir.
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	state := &model.HookState{
		Cwd:         "/home/user/project",
		PID:         1234,
		SessionID:   "test-session-1",
		Event:       "PreToolUse",
		Status:      model.StatusWorking,
		LastEventAt: time.Now(),
		ToolName:    "Bash",
		Sequence:    1,
		UpdatedBy:   "ttyrant hook",
	}

	if err := WriteState(state); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	// Read back and verify.
	path := StateFilePath("/home/user/project")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}

	var got model.HookState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Cwd != state.Cwd {
		t.Errorf("Cwd = %q, want %q", got.Cwd, state.Cwd)
	}
	if got.PID != state.PID {
		t.Errorf("PID = %d, want %d", got.PID, state.PID)
	}
	if got.Status != model.StatusWorking {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusWorking)
	}
	if got.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", got.ToolName, "Bash")
	}
}

func TestWriteState_Overwrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	state1 := &model.HookState{
		Cwd:       "/home/user/project",
		PID:       1234,
		SessionID: "s1",
		Event:     "SessionStart",
		Status:    model.StatusStarting,
		Sequence:  1,
	}
	state2 := &model.HookState{
		Cwd:       "/home/user/project",
		PID:       1234,
		SessionID: "s1",
		Event:     "PreToolUse",
		Status:    model.StatusWorking,
		Sequence:  2,
	}

	if err := WriteState(state1); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state2); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(StateFilePath("/home/user/project"))
	if err != nil {
		t.Fatal(err)
	}

	var got model.HookState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.Status != model.StatusWorking {
		t.Errorf("Status = %q after overwrite, want %q", got.Status, model.StatusWorking)
	}
	if got.Sequence != 2 {
		t.Errorf("Sequence = %d, want 2", got.Sequence)
	}
}

func TestRemoveState(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	state := &model.HookState{
		Cwd:    "/home/user/project",
		PID:    1234,
		Status: model.StatusWorking,
	}
	if err := WriteState(state); err != nil {
		t.Fatal(err)
	}

	if err := RemoveState("/home/user/project"); err != nil {
		t.Fatalf("RemoveState: %v", err)
	}

	path := StateFilePath("/home/user/project")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("state file should be removed")
	}

	// Removing non-existent should not error.
	if err := RemoveState("/nonexistent"); err != nil {
		t.Errorf("RemoveState non-existent: %v", err)
	}
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Verify directories exist.
	for _, dir := range []string{
		filepath.Join(tmp, "ttyrant", "current"),
		filepath.Join(tmp, "ttyrant", "events"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("dir %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestStateFilePath_Deterministic(t *testing.T) {
	p1 := StateFilePath("/home/user/project-a")
	p2 := StateFilePath("/home/user/project-a")
	if p1 != p2 {
		t.Error("same cwd should produce same state file path")
	}

	p3 := StateFilePath("/home/user/project-b")
	if p1 == p3 {
		t.Error("different cwds should produce different state file paths")
	}
}
