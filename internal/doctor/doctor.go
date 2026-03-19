package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pfnilsson/ttyrant/internal/install"
	"github.com/pfnilsson/ttyrant/internal/scanner"
	"github.com/pfnilsson/ttyrant/internal/state"
)

// CheckResult represents the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string
	OK      bool
	Message string
}

// RunAll runs all diagnostic checks and returns the results.
func RunAll() []CheckResult {
	return []CheckResult{
		checkHookBinary(),
		checkStateDir(),
		checkStateReadable(),
		checkHooksInstalled(),
		checkScanner(),
		checkClaudeProcesses(),
	}
}

// Print formats and prints all check results, returning true if all passed.
func Print(results []CheckResult) bool {
	allOK := true
	for _, r := range results {
		icon := "✓"
		if !r.OK {
			icon = "✗"
			allOK = false
		}
		fmt.Printf("  %s %s: %s\n", icon, r.Name, r.Message)
	}
	return allOK
}

func checkHookBinary() CheckResult {
	name := "ttyrant in PATH"
	path, err := exec.LookPath("ttyrant")
	if err != nil {
		return CheckResult{name, false, "not found — install ttyrant and ensure it's in PATH"}
	}
	return CheckResult{name, true, path}
}

func checkStateDir() CheckResult {
	name := "State directory"
	dir := state.CurrentDir()

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to create it.
			if err := state.EnsureDirs(); err != nil {
				return CheckResult{name, false, fmt.Sprintf("cannot create %s: %v", dir, err)}
			}
			return CheckResult{name, true, fmt.Sprintf("%s (created)", dir)}
		}
		return CheckResult{name, false, fmt.Sprintf("cannot access %s: %v", dir, err)}
	}
	if !info.IsDir() {
		return CheckResult{name, false, fmt.Sprintf("%s exists but is not a directory", dir)}
	}

	// Test writability.
	tmp, err := os.CreateTemp(dir, ".doctor-test-*")
	if err != nil {
		return CheckResult{name, false, fmt.Sprintf("%s is not writable: %v", dir, err)}
	}
	tmp.Close()
	os.Remove(tmp.Name())

	return CheckResult{name, true, dir}
}

func checkStateReadable() CheckResult {
	name := "State files readable"
	states, err := state.ReadAllStates()
	if err != nil {
		return CheckResult{name, false, fmt.Sprintf("error reading state files: %v", err)}
	}
	return CheckResult{name, true, fmt.Sprintf("%d state file(s) found", len(states))}
}

func checkHooksInstalled() CheckResult {
	name := "Hooks in Claude Code settings"
	if install.IsInstalled() {
		return CheckResult{name, true, "all hook events registered"}
	}
	return CheckResult{name, false, "hooks not installed — run `ttyrant install-hooks`"}
}

func checkScanner() CheckResult {
	name := "Process scanner"
	s := scanner.New()
	_, err := s.Scan(context.Background())
	if err != nil {
		return CheckResult{name, false, fmt.Sprintf("scanner error: %v", err)}
	}
	return CheckResult{name, true, "working"}
}

func checkClaudeProcesses() CheckResult {
	name := "Claude Code processes"
	s := scanner.New()
	procs, err := s.Scan(context.Background())
	if err != nil {
		return CheckResult{name, false, fmt.Sprintf("scanner error: %v", err)}
	}
	if len(procs) == 0 {
		return CheckResult{name, true, "none running (this is OK)"}
	}

	var dirs []string
	for _, p := range procs {
		dirs = append(dirs, p.Cwd)
	}
	return CheckResult{name, true, fmt.Sprintf("%d found: %s", len(procs), strings.Join(dirs, ", "))}
}
