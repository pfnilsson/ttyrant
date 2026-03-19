package scanner

import (
	"os"
	"testing"
)

func TestParseProcessList_Linux(t *testing.T) {
	data, err := os.ReadFile("../../testdata/ps/linux_output.txt")
	if err != nil {
		t.Fatal(err)
	}

	entries := parseProcessList(string(data))

	if len(entries) != 8 {
		t.Fatalf("got %d entries, want 8", len(entries))
	}

	// Spot-check first entry.
	if entries[0].pid != 1234 {
		t.Errorf("entry 0 PID = %d, want 1234", entries[0].pid)
	}
	if entries[0].ppid != 1000 {
		t.Errorf("entry 0 PPID = %d, want 1000", entries[0].ppid)
	}
	if entries[0].tty != "pts/0" {
		t.Errorf("entry 0 TTY = %q, want pts/0", entries[0].tty)
	}
	if entries[0].startTime.IsZero() {
		t.Error("entry 0 start time should not be zero")
	}
}

func TestParseProcessList_Darwin(t *testing.T) {
	data, err := os.ReadFile("../../testdata/ps/darwin_output.txt")
	if err != nil {
		t.Fatal(err)
	}

	entries := parseProcessList(string(data))

	// Header should be skipped, leaving 4 data lines.
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}

	if entries[0].pid != 1234 {
		t.Errorf("entry 0 PID = %d, want 1234", entries[0].pid)
	}
}

func TestParsePSLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantPID int
		wantOK  bool
	}{
		{
			name:    "valid line",
			line:    " 1234  1000 pts/0   Mon Mar 17 10:00:00 2025 node /usr/bin/claude",
			wantPID: 1234,
			wantOK:  true,
		},
		{
			name:    "too short",
			line:    "1234 1000",
			wantPID: 0,
			wantOK:  false,
		},
		{
			name:    "header line",
			line:    "  PID  PPID TTY      STARTED COMMAND",
			wantPID: 0,
			wantOK:  false,
		},
		{
			name:    "empty",
			line:    "",
			wantPID: 0,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := parsePSLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && e.pid != tt.wantPID {
				t.Errorf("PID = %d, want %d", e.pid, tt.wantPID)
			}
		})
	}
}

func TestIsClaudeProcess(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{
			"anthropic-ai package",
			"node /home/user/.claude/local/node_modules/@anthropic-ai/claude-code/cli.js",
			true,
		},
		{
			"claude binary",
			"node /usr/local/bin/claude",
			true,
		},
		{
			"vim not claude",
			"vim /home/user/project/main.go",
			false,
		},
		{
			"bash not claude",
			"bash",
			false,
		},
		{
			"grep claude excluded",
			"grep claude",
			false,
		},
		{
			"ttyrant excluded",
			"/usr/bin/ttyrant scan --json",
			false,
		},
		{
			"claude-code index",
			"node /home/user/.local/share/claude-code/node_modules/@anthropic-ai/claude-code/index.js",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := processEntry{cmdline: tt.cmd}
			if got := isClaudeProcess(e); got != tt.want {
				t.Errorf("isClaudeProcess(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}
