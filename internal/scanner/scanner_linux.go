//go:build linux

package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// listProcesses runs ps to get all processes with relevant columns.
func listProcesses(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ps", "-eo", "pid,ppid,tty,lstart,args", "--no-headers")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	return string(out), nil
}

// resolveWorkingDir resolves the working directory of a process.
// On Linux, reads /proc/<pid>/cwd symlink first, then falls back to lsof.
func resolveWorkingDir(ctx context.Context, pid int) string {
	// Try /proc/PID/cwd first (fast, no subprocess).
	link := fmt.Sprintf("/proc/%d/cwd", pid)
	target, err := os.Readlink(link)
	if err == nil && target != "" {
		return target
	}

	// Fallback to lsof.
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

	// lsof output has lines starting with 'n' for the name field.
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(line, "n") && len(line) > 1 {
			return line[1:]
		}
	}
	return ""
}
