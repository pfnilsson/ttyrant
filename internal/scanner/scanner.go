package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/util"
)

// Scanner discovers running Claude Code processes.
type Scanner struct{}

// New creates a new Scanner.
func New() *Scanner {
	return &Scanner{}
}

// Scan discovers all running Claude Code processes and returns them as LiveProcess entries.
func (s *Scanner) Scan(ctx context.Context) ([]model.LiveProcess, error) {
	raw, err := listProcesses(ctx)
	if err != nil {
		// ps failure is non-fatal — return empty list.
		return nil, nil
	}

	entries := parseProcessList(raw)
	now := time.Now()

	var results []model.LiveProcess
	for _, e := range entries {
		if !isClaudeProcess(e) {
			continue
		}

		cwd := resolveWorkingDir(ctx, e.pid)
		if cwd == "" {
			continue
		}
		cwd = util.NormalizePath(cwd)

		transport := "pty"
		if e.tty == "?" || e.tty == "" {
			transport = "unknown"
		}

		results = append(results, model.LiveProcess{
			PID:        e.pid,
			PPID:       e.ppid,
			Cmdline:    e.cmdline,
			Cwd:        cwd,
			TTY:        e.tty,
			StartTime:  e.startTime,
			LastSeenAt: now,
			Transport:  transport,
		})
	}

	return results, nil
}

// processEntry is a raw parsed process from ps output.
type processEntry struct {
	pid       int
	ppid      int
	cmdline   string
	tty       string
	startTime time.Time
}

// isClaudeProcess determines if a process entry is a Claude Code interactive session.
// We look for the Node.js process running the Claude CLI.
func isClaudeProcess(e processEntry) bool {
	cmd := e.cmdline

	// Claude Code runs as a Node.js process. The command line typically contains
	// the path to the claude CLI entry point.
	// Common patterns:
	//   node /path/to/.claude/local/node_modules/.bin/claude
	//   /path/to/node /path/to/claude
	//   claude (if installed globally and resolved)

	lower := strings.ToLower(cmd)

	// Must contain "claude" somewhere in the command.
	if !strings.Contains(lower, "claude") {
		return false
	}

	// Exclude our own processes.
	if strings.Contains(lower, "ttyrant") {
		return false
	}

	// Exclude grep/ps processes that happen to match.
	if strings.HasPrefix(lower, "grep") || strings.HasPrefix(lower, "rg ") {
		return false
	}

	// Look for patterns that indicate an interactive Claude Code session.
	// The main Claude Code process typically has one of these in its command line:
	indicators := []string{
		"@anthropic-ai/claude-code",  // npm package path
		"bin/claude",                 // CLI binary path
		"claude-code",               // package name
	}

	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}

	// Also match if the command is just "claude" with node.
	if strings.Contains(lower, "node") && strings.Contains(cmd, "claude") {
		return true
	}

	return false
}
