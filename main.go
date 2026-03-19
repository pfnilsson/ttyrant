package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pfnilsson/ttyrant/internal/doctor"
	"github.com/pfnilsson/ttyrant/internal/hooks"
	"github.com/pfnilsson/ttyrant/internal/install"
	"github.com/pfnilsson/ttyrant/internal/scanner"
	"github.com/pfnilsson/ttyrant/internal/tui"
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "hook":
		runHook()
	case "scan":
		runScan()
	case "install-hooks":
		runInstallHooks()
	case "uninstall-hooks":
		runUninstallHooks()
	case "doctor":
		runDoctor()
	default:
		fmt.Fprintf(os.Stderr, "Usage: ttyrant [command]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  hook              Process a Claude Code hook event (used by hooks config)\n")
		fmt.Fprintf(os.Stderr, "  scan              Scan for Claude Code processes\n")
		fmt.Fprintf(os.Stderr, "  install-hooks     Install ttyrant hooks into Claude Code\n")
		fmt.Fprintf(os.Stderr, "  uninstall-hooks   Remove ttyrant hooks from Claude Code\n")
		fmt.Fprintf(os.Stderr, "  doctor            Run diagnostic checks\n")
		fmt.Fprintf(os.Stderr, "\nRun with no arguments to launch the TUI dashboard.\n")
		os.Exit(1)
	}
}

func runTUI() {
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func runScan() {
	jsonOutput := false
	for _, arg := range os.Args[2:] {
		if arg == "--json" {
			jsonOutput = true
		}
	}

	s := scanner.New()
	procs, err := s.Scan(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(procs); err != nil {
			fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
			os.Exit(1)
		}
	} else {
		if len(procs) == 0 {
			fmt.Println("No Claude Code sessions found.")
			return
		}
		for _, p := range procs {
			fmt.Printf("PID=%d  CWD=%s  TTY=%s  Transport=%s\n", p.PID, p.Cwd, p.TTY, p.Transport)
			fmt.Printf("  CMD: %s\n", p.Cmdline)
			fmt.Println()
		}
	}
}

func runInstallHooks() {
	printOnly := false
	for _, arg := range os.Args[2:] {
		if arg == "--print" {
			printOnly = true
		}
	}

	if err := install.Install(printOnly); err != nil {
		fmt.Fprintf(os.Stderr, "install-hooks error: %v\n", err)
		os.Exit(1)
	}

	if !printOnly {
		fmt.Println("Hooks installed successfully.")
		fmt.Println("Run `ttyrant doctor` to verify the setup.")
	}
}

func runUninstallHooks() {
	if err := install.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "uninstall-hooks error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Hooks removed.")
}

func runHook() {
	pid := hooks.GetPIDFromEnv()
	if err := hooks.ProcessHookEvent(os.Stdin, pid); err != nil {
		fmt.Fprintf(os.Stderr, "ttyrant hook: %v\n", err)
		os.Exit(1)
	}
}

func runDoctor() {
	fmt.Println("ttyrant doctor")
	fmt.Println()
	results := doctor.RunAll()
	allOK := doctor.Print(results)
	fmt.Println()
	if allOK {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed. See above for details.")
		os.Exit(1)
	}
}
