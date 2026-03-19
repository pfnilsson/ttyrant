package scanner

import (
	"strconv"
	"strings"
	"time"
)

// parseProcessList parses the output of `ps` into process entries.
// Expected format: PID PPID TTY LSTART COMMAND
// LSTART is a multi-word timestamp like "Mon Jan  2 15:04:05 2006".
func parseProcessList(output string) []processEntry {
	lines := strings.Split(output, "\n")
	var entries []processEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip header line.
		if strings.HasPrefix(line, "PID") || strings.HasPrefix(line, "  PID") {
			continue
		}

		e, ok := parsePSLine(line)
		if ok {
			entries = append(entries, e)
		}
	}

	return entries
}

// parsePSLine parses a single line of ps output.
// Format: PID PPID TTY LSTART(5 words) COMMAND(rest)
// Example: 12345 12344 pts/0 Mon Jan  2 15:04:05 2006 node /usr/bin/claude
func parsePSLine(line string) (processEntry, bool) {
	fields := strings.Fields(line)
	// Minimum: PID PPID TTY + 5 LSTART words + at least 1 command word = 9
	if len(fields) < 9 {
		return processEntry{}, false
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return processEntry{}, false
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return processEntry{}, false
	}

	tty := fields[2]

	// LSTART is 5 words: day-of-week month day time year
	// e.g., "Mon Jan  2 15:04:05 2006"
	lstartStr := strings.Join(fields[3:8], " ")
	startTime := parseLSTART(lstartStr)

	// Everything after LSTART is the command.
	cmdline := strings.Join(fields[8:], " ")

	return processEntry{
		pid:       pid,
		ppid:      ppid,
		tty:       tty,
		startTime: startTime,
		cmdline:   cmdline,
	}, true
}

// parseLSTART parses the LSTART format from ps.
// Format: "Mon Jan  2 15:04:05 2006"
func parseLSTART(s string) time.Time {
	// ps LSTART format uses C locale.
	layouts := []string{
		"Mon Jan  2 15:04:05 2006",
		"Mon Jan 2 15:04:05 2006",
		time.ANSIC,
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}
