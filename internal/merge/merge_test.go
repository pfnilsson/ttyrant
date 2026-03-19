package merge

import (
	"testing"
	"time"

	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/tmux"
)

func sessions(names ...string) []tmux.Session {
	var s []tmux.Session
	for _, n := range names {
		s = append(s, tmux.Session{Name: n, Path: "/" + n})
	}
	return s
}

func TestMerge_TmuxSessionWithClaude(t *testing.T) {
	now := time.Now()
	sess := sessions("project")
	procs := []model.LiveProcess{
		{PID: 100, Cwd: "/project", Cmdline: "node claude", StartTime: now.Add(-30 * time.Second)},
	}
	hooks := []model.HookState{
		{Cwd: "/project", PID: 100, Event: "PreToolUse", Status: model.StatusWorking, LastEventAt: now.Add(-2 * time.Second), ToolName: "Bash"},
	}

	rows := Merge(sess, procs, hooks, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.SessionName != "project" {
		t.Errorf("SessionName = %q, want project", r.SessionName)
	}
	if r.Status != model.StatusWorking {
		t.Errorf("Status = %q, want working", r.Status)
	}
	if r.StatusSource != model.SourceHooks {
		t.Errorf("StatusSource = %q, want hooks", r.StatusSource)
	}
	if !r.HasClaude {
		t.Error("expected HasClaude = true")
	}
	if r.PID != 100 {
		t.Errorf("PID = %d, want 100", r.PID)
	}
}

func TestMerge_TmuxSessionWithoutClaude(t *testing.T) {
	now := time.Now()
	sess := sessions("myproject")

	rows := Merge(sess, nil, nil, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.Status != model.StatusActive {
		t.Errorf("Status = %q, want active", r.Status)
	}
	if r.HasClaude {
		t.Error("expected HasClaude = false")
	}
	if r.SessionName != "myproject" {
		t.Errorf("SessionName = %q, want myproject", r.SessionName)
	}
}

func TestMerge_NeedsInput(t *testing.T) {
	now := time.Now()
	sess := sessions("project")
	procs := []model.LiveProcess{
		{PID: 100, Cwd: "/project", StartTime: now.Add(-1 * time.Minute)},
	}
	hooks := []model.HookState{
		{Cwd: "/project", PID: 100, Event: "PermissionRequest", Status: model.StatusNeedsInput, LastEventAt: now.Add(-5 * time.Second), WaitingReason: "permission"},
	}

	rows := Merge(sess, procs, hooks, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Status != model.StatusNeedsInput {
		t.Errorf("Status = %q, want needs_input", rows[0].Status)
	}
	if rows[0].WaitingReason != "permission" {
		t.Errorf("WaitingReason = %q, want permission", rows[0].WaitingReason)
	}
}

func TestMerge_ClaudeStarting(t *testing.T) {
	now := time.Now()
	sess := sessions("new-project")
	procs := []model.LiveProcess{
		{PID: 200, Cwd: "/new-project", StartTime: now.Add(-3 * time.Second)},
	}

	rows := Merge(sess, procs, nil, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Status != model.StatusStarting {
		t.Errorf("Status = %q, want starting", rows[0].Status)
	}
	if !rows[0].HasClaude {
		t.Error("expected HasClaude = true")
	}
}

func TestMerge_ClaudeUnknown(t *testing.T) {
	now := time.Now()
	sess := sessions("old-project")
	procs := []model.LiveProcess{
		{PID: 200, Cwd: "/old-project", StartTime: now.Add(-30 * time.Second)},
	}

	rows := Merge(sess, procs, nil, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Status != model.StatusUnknown {
		t.Errorf("Status = %q, want unknown", rows[0].Status)
	}
}

func TestMerge_StaleHook(t *testing.T) {
	now := time.Now()
	sess := sessions("project")
	procs := []model.LiveProcess{
		{PID: 100, Cwd: "/project", StartTime: now.Add(-10 * time.Minute)},
	}
	hooks := []model.HookState{
		{Cwd: "/project", PID: 100, Event: "PreToolUse", Status: model.StatusWorking, LastEventAt: now.Add(-6 * time.Minute)},
	}

	rows := Merge(sess, procs, hooks, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Status != model.StatusUnknown {
		t.Errorf("Status = %q, want unknown (stale)", rows[0].Status)
	}
	if rows[0].Warning == "" {
		t.Error("expected warning for stale hook state")
	}
}

func TestMerge_DeadClaudeShowsActive(t *testing.T) {
	now := time.Now()
	sess := sessions("project")
	hooks := []model.HookState{
		{Cwd: "/project", PID: 300, Event: "SessionEnd", Status: model.StatusExited, LastEventAt: now.Add(-1 * time.Minute)},
	}

	rows := Merge(sess, nil, hooks, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Status != model.StatusActive {
		t.Errorf("Status = %q, want active (no live claude)", rows[0].Status)
	}
	if rows[0].HasClaude {
		t.Error("expected HasClaude = false when no live process")
	}
}

func TestMerge_MultipleSessions(t *testing.T) {
	now := time.Now()
	sess := []tmux.Session{
		{Name: "project-a", Path: "/project-a"},
		{Name: "project-b", Path: "/project-b"},
		{Name: "project-c", Path: "/project-c"},
	}
	procs := []model.LiveProcess{
		{PID: 100, Cwd: "/project-a", StartTime: now.Add(-1 * time.Minute)},
	}
	hooks := []model.HookState{
		{Cwd: "/project-a", PID: 100, Event: "PreToolUse", Status: model.StatusWorking, LastEventAt: now.Add(-1 * time.Second)},
	}

	rows := Merge(sess, procs, hooks, now)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}

	byName := make(map[string]model.SessionRow)
	for _, r := range rows {
		byName[r.SessionName] = r
	}

	if r := byName["project-a"]; r.Status != model.StatusWorking {
		t.Errorf("project-a Status = %q, want working", r.Status)
	}
	if r := byName["project-b"]; r.Status != model.StatusActive {
		t.Errorf("project-b Status = %q, want active", r.Status)
	}
	if r := byName["project-c"]; r.Status != model.StatusActive {
		t.Errorf("project-c Status = %q, want active", r.Status)
	}
}

func TestMerge_ChildDirMatch(t *testing.T) {
	now := time.Now()
	sess := []tmux.Session{
		{Name: "myproject", Path: "/home/user/myproject"},
	}
	procs := []model.LiveProcess{
		{PID: 100, Cwd: "/home/user/myproject/subdir", StartTime: now.Add(-1 * time.Minute)},
	}
	hooks := []model.HookState{
		{Cwd: "/home/user/myproject/subdir", PID: 100, Event: "PreToolUse", Status: model.StatusWorking, LastEventAt: now.Add(-1 * time.Second)},
	}

	rows := Merge(sess, procs, hooks, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Status != model.StatusWorking {
		t.Errorf("Status = %q, want working", rows[0].Status)
	}
	if !rows[0].HasClaude {
		t.Error("expected HasClaude = true via child dir match")
	}
}

func TestMerge_IdleFor(t *testing.T) {
	now := time.Now()
	sess := sessions("project")
	procs := []model.LiveProcess{
		{PID: 100, Cwd: "/project", StartTime: now.Add(-5 * time.Minute)},
	}
	eventTime := now.Add(-90 * time.Second)
	hooks := []model.HookState{
		{Cwd: "/project", PID: 100, Event: "PreToolUse", Status: model.StatusWorking, LastEventAt: eventTime},
	}

	rows := Merge(sess, procs, hooks, now)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].IdleFor < 89*time.Second || rows[0].IdleFor > 91*time.Second {
		t.Errorf("IdleFor = %v, want ~90s", rows[0].IdleFor)
	}
}

func TestMerge_NoTmuxSessions(t *testing.T) {
	now := time.Now()
	rows := Merge(nil, nil, nil, now)
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0", len(rows))
	}
}
