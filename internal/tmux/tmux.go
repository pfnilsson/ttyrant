package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Session represents a tmux session.
type Session struct {
	Name string
	Path string
}

// ListSessions returns all active tmux sessions.
func ListSessions() []Session {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\t#{session_path}").Output()
	if err != nil {
		return nil
	}

	var sessions []Session
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		sessions = append(sessions, Session{
			Name: parts[0],
			Path: filepath.Clean(parts[1]),
		})
	}
	return sessions
}

// FindSession finds the tmux session whose start directory matches (or is a
// parent of) the given cwd. Returns the session name, or "" if none found.
func FindSession(cwd string) string {
	cwd = filepath.Clean(cwd)
	var bestName string
	bestLen := 0

	for _, s := range ListSessions() {
		if s.Path == cwd {
			return s.Name
		}
		if strings.HasPrefix(cwd, s.Path+"/") && len(s.Path) > bestLen {
			bestName = s.Name
			bestLen = len(s.Path)
		}
	}

	return bestName
}

// CurrentSession returns the name of the tmux session that the ttyrant
// client belongs to. When running inside a tmux popup, TTYRANT_TMUX_CLIENT
// identifies the parent client; otherwise we fall back to the TMUX env var.
func CurrentSession() string {
	if client := os.Getenv("TTYRANT_TMUX_CLIENT"); client != "" {
		out, err := exec.Command("tmux", "display-message", "-t", client, "-p", "#{session_name}").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// KillSession kills a tmux session by name. If the target is the current
// session, it switches the client to another session first so tmux doesn't
// detach entirely.
func KillSession(name string) {
	if InsideTmux() && CurrentSession() == name {
		// Switch to the previous session (or any other) before killing.
		_ = exec.Command("tmux", "switch-client", "-l").Run()
	}
	_ = exec.Command("tmux", "kill-session", "-t", "="+name).Run()
}

// InsideTmux returns true if we're running inside a tmux session.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// CreateWorktreeSession creates a new tmux session for a worktree, matching
// the tmuxrun convention: window 1 = nvim, window 2 = terminal, with
// @repo_name and @branch session variables.
func CreateWorktreeSession(name, worktreePath, repoName, branch string) error {
	if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", worktreePath, "-n", "nvim").Run(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	_ = exec.Command("tmux", "send-keys", "-t", name+":1", "nvim", "Enter").Run()
	_ = exec.Command("tmux", "new-window", "-t", name, "-c", worktreePath, "-n", "terminal").Run()
	_ = exec.Command("tmux", "set-option", "-t", name, "@repo_name", repoName).Run()
	_ = exec.Command("tmux", "set-option", "-t", name, "@branch", branch).Run()
	_ = exec.Command("tmux", "select-window", "-t", name+":1").Run()
	return nil
}

// HasSession checks whether a tmux session with the given name exists.
func HasSession(name string) bool {
	return exec.Command("tmux", "has-session", "-t", "="+name).Run() == nil
}

// CreateSession creates a new tmux session for a regular (non-worktree) project.
// Window 1 = nvim, window 2 = terminal, with @repo_name and @branch set.
func CreateSession(name, path string) error {
	if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", path, "-n", "nvim").Run(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	_ = exec.Command("tmux", "send-keys", "-t", name+":1", "nvim", "Enter").Run()
	_ = exec.Command("tmux", "new-window", "-t", name, "-c", path, "-n", "terminal").Run()
	_ = exec.Command("tmux", "set-option", "-t", name, "@repo_name", filepath.Base(path)).Run()
	if out, err := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" && branch != "HEAD" {
			_ = exec.Command("tmux", "set-option", "-t", name, "@branch", branch).Run()
		}
	}
	_ = exec.Command("tmux", "select-window", "-t", name+":1").Run()
	return nil
}

// AttachSessionCmd returns the command to attach/switch to a tmux session
// without targeting a specific window (resumes wherever it was left off).
func AttachSessionCmd(name string) *exec.Cmd {
	target := "=" + name
	if InsideTmux() {
		args := []string{"switch-client", "-t", target}
		if client := os.Getenv("TTYRANT_TMUX_CLIENT"); client != "" {
			args = append(args, "-c", client)
		}
		return exec.Command("tmux", args...)
	}
	return exec.Command("tmux", "attach-session", "-t", target)
}

// AttachCmd returns the command to attach/switch to a tmux session at the given window (1-based).
// If TTYRANT_TMUX_CLIENT is set (e.g. when running inside a tmux popup),
// switch-client targets that client so the parent session switches and the popup closes.
func AttachCmd(name string, window int) *exec.Cmd {
	target := fmt.Sprintf("=%s:%d", name, window)
	if InsideTmux() {
		args := []string{"switch-client", "-t", target}
		if client := os.Getenv("TTYRANT_TMUX_CLIENT"); client != "" {
			args = append(args, "-c", client)
		}
		return exec.Command("tmux", args...)
	}
	return exec.Command("tmux", "attach-session", "-t", target)
}
