//go:build darwin

package scanner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// listProcesses runs ps to get all processes with relevant columns.
func listProcesses(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// macOS ps uses slightly different flags — no --no-headers, but we can skip headers in parsing.
	cmd := exec.CommandContext(ctx, "ps", "-eo", "pid,ppid,tty,lstart,command")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	return string(out), nil
}

// resolveWorkingDir resolves the working directory of a process.
// On macOS, /proc doesn't exist, so we use lsof.
func resolveWorkingDir(ctx context.Context, pid int) string {
	return resolveCwdWithLsof(ctx, pid)
}

// resolveCwdWithLsof uses lsof -p PID to find the cwd.
func resolveCwdWithLsof(ctx context.Context, pid int) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lsof", "-p", fmt.Sprintf("%d", pid), "-Fn", "-a", "-d", "cwd")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") && len(line) > 1 {
			return line[1:]
		}
	}
	return ""
}
