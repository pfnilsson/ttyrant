package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/state"
)

func TestProcessHookEvent_SessionStart(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "SessionStart",
		"permission_mode": "default"
	}`

	err := ProcessHookEvent(strings.NewReader(payload), 1234)
	if err != nil {
		t.Fatalf("ProcessHookEvent: %v", err)
	}

	s, err := state.ReadStateFile("/home/user/project")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected state file to be created")
	}
	if s.Status != model.StatusStarting {
		t.Errorf("Status = %q, want %q", s.Status, model.StatusStarting)
	}
	if s.Event != "SessionStart" {
		t.Errorf("Event = %q, want SessionStart", s.Event)
	}
	if s.PID != 1234 {
		t.Errorf("PID = %d, want 1234", s.PID)
	}
	if s.SessionID != "s1" {
		t.Errorf("SessionID = %q, want s1", s.SessionID)
	}
}

func TestProcessHookEvent_PermissionRequest(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "PermissionRequest",
		"permission_mode": "default",
		"tool_name": "Bash"
	}`

	if err := ProcessHookEvent(strings.NewReader(payload), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/home/user/project")
	if s.Status != model.StatusNeedsInput {
		t.Errorf("Status = %q, want needs_input", s.Status)
	}
	if s.WaitingReason != "permission" {
		t.Errorf("WaitingReason = %q, want permission", s.WaitingReason)
	}
	if s.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", s.ToolName)
	}
}

func TestProcessHookEvent_Elicitation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "Elicitation",
		"permission_mode": "default"
	}`

	if err := ProcessHookEvent(strings.NewReader(payload), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/home/user/project")
	if s.Status != model.StatusNeedsInput {
		t.Errorf("Status = %q, want needs_input", s.Status)
	}
	if s.WaitingReason != "elicitation" {
		t.Errorf("WaitingReason = %q, want elicitation", s.WaitingReason)
	}
}

func TestProcessHookEvent_ToolUse(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "PreToolUse",
		"permission_mode": "default",
		"tool_name": "Write"
	}`

	if err := ProcessHookEvent(strings.NewReader(payload), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/home/user/project")
	if s.Status != model.StatusWorking {
		t.Errorf("Status = %q, want working", s.Status)
	}
}

func TestProcessHookEvent_TaskCompleted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "TaskCompleted",
		"permission_mode": "default"
	}`

	if err := ProcessHookEvent(strings.NewReader(payload), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/home/user/project")
	if s.Status != model.StatusDone {
		t.Errorf("Status = %q, want done", s.Status)
	}
}

func TestProcessHookEvent_SessionEnd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "SessionEnd",
		"permission_mode": "default"
	}`

	if err := ProcessHookEvent(strings.NewReader(payload), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/home/user/project")
	if s.Status != model.StatusExited {
		t.Errorf("Status = %q, want exited", s.Status)
	}
}

func TestProcessHookEvent_NotificationIgnored(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	// Set initial state to working.
	p1 := `{"session_id":"s1","cwd":"/home/user/project","hook_event_name":"PreToolUse","tool_name":"Bash"}`
	if err := ProcessHookEvent(strings.NewReader(p1), 1234); err != nil {
		t.Fatal(err)
	}

	// Notification should be completely ignored — status stays working.
	p2 := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "Notification",
		"notification_type": "permission_prompt"
	}`
	if err := ProcessHookEvent(strings.NewReader(p2), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/home/user/project")
	if s.Status != model.StatusWorking {
		t.Errorf("Status = %q, want working (Notification should be ignored)", s.Status)
	}
}

func TestProcessHookEvent_EventLog(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	payload := `{
		"session_id": "s1",
		"cwd": "/home/user/project",
		"hook_event_name": "SessionStart",
		"permission_mode": "default"
	}`

	if err := ProcessHookEvent(strings.NewReader(payload), 1234); err != nil {
		t.Fatal(err)
	}

	// Verify event log file was created.
	entries, err := os.ReadDir(filepath.Join(tmp, "ttyrant", "events"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 event log file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(tmp, "ttyrant", "events", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "SessionStart") {
		t.Error("event log should contain SessionStart")
	}
}

func TestProcessHookEvent_MissingCwd(t *testing.T) {
	payload := `{"session_id": "s1", "hook_event_name": "SessionStart"}`
	err := ProcessHookEvent(strings.NewReader(payload), 1234)
	if err == nil {
		t.Error("expected error for missing cwd")
	}
}

func TestProcessHookEvent_MissingEvent(t *testing.T) {
	payload := `{"session_id": "s1", "cwd": "/foo"}`
	err := ProcessHookEvent(strings.NewReader(payload), 1234)
	if err == nil {
		t.Error("expected error for missing hook_event_name")
	}
}

func TestProcessHookEvent_InvalidJSON(t *testing.T) {
	err := ProcessHookEvent(strings.NewReader("not json"), 1234)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestProcessHookEvent_SequenceProgression(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	// Send two events in order — second should win.
	p1 := `{"session_id":"s1","cwd":"/project","hook_event_name":"SessionStart"}`
	p2 := `{"session_id":"s1","cwd":"/project","hook_event_name":"PreToolUse","tool_name":"Bash"}`

	if err := ProcessHookEvent(strings.NewReader(p1), 1234); err != nil {
		t.Fatal(err)
	}
	if err := ProcessHookEvent(strings.NewReader(p2), 1234); err != nil {
		t.Fatal(err)
	}

	s, _ := state.ReadStateFile("/project")
	if s.Status != model.StatusWorking {
		t.Errorf("Status = %q, want working", s.Status)
	}
}

func TestProcessHookEvent_IdleNotificationAfterDone(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	// Task completes → status is done.
	p1 := `{"session_id":"s1","cwd":"/project","hook_event_name":"TaskCompleted"}`
	if err := ProcessHookEvent(strings.NewReader(p1), 1234); err != nil {
		t.Fatal(err)
	}
	s, _ := state.ReadStateFile("/project")
	if s.Status != model.StatusDone {
		t.Fatalf("Status = %q, want done", s.Status)
	}

	// Idle notification arrives — should NOT override done.
	p2 := `{"session_id":"s1","cwd":"/project","hook_event_name":"Notification","notification_type":"idle_prompt"}`
	if err := ProcessHookEvent(strings.NewReader(p2), 1234); err != nil {
		t.Fatal(err)
	}
	s, _ = state.ReadStateFile("/project")
	if s.Status != model.StatusDone {
		t.Errorf("Status = %q, want done (idle_prompt should not override)", s.Status)
	}
}

func TestProcessHookEvent_NewPromptAfterDone(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	// Task completes.
	p1 := `{"session_id":"s1","cwd":"/project","hook_event_name":"TaskCompleted"}`
	if err := ProcessHookEvent(strings.NewReader(p1), 1234); err != nil {
		t.Fatal(err)
	}

	// User submits a new prompt — should override done.
	p2 := `{"session_id":"s1","cwd":"/project","hook_event_name":"UserPromptSubmit"}`
	if err := ProcessHookEvent(strings.NewReader(p2), 1234); err != nil {
		t.Fatal(err)
	}
	s, _ := state.ReadStateFile("/project")
	if s.Status != model.StatusWorking {
		t.Errorf("Status = %q, want working", s.Status)
	}
}

func TestProcessHookEvent_FixtureFiles(t *testing.T) {
	fixtures := []struct {
		file   string
		status model.SessionStatus
	}{
		{"../../testdata/hooks/session_start.json", model.StatusStarting},
		{"../../testdata/hooks/tool_use.json", model.StatusWorking},
		{"../../testdata/hooks/permission_request.json", model.StatusNeedsInput},
		{"../../testdata/hooks/task_completed.json", model.StatusDone},
		{"../../testdata/hooks/session_end.json", model.StatusExited},
	}

	for _, tt := range fixtures {
		t.Run(filepath.Base(tt.file), func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("XDG_STATE_HOME", tmp)

			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatal(err)
			}

			if err := ProcessHookEvent(strings.NewReader(string(data)), 1234); err != nil {
				t.Fatal(err)
			}

			s, _ := state.ReadStateFile("/home/user/my-project")
			if s == nil {
				t.Fatal("expected state file")
			}
			if s.Status != tt.status {
				t.Errorf("Status = %q, want %q", s.Status, tt.status)
			}
		})
	}
}
