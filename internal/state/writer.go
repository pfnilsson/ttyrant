package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pfnilsson/ttyrant/internal/model"
	"github.com/pfnilsson/ttyrant/internal/util"
)

const (
	// DefaultStateDir is the base directory for ttyrant state files.
	stateDirName = "ttyrant"
	currentDir   = "current"
	eventsDir    = "events"
)

// StateDir returns the base state directory (~/.local/state/ttyrant).
func StateDir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, stateDirName)
}

// CurrentDir returns the directory for current-state JSON files.
func CurrentDir() string {
	return filepath.Join(StateDir(), currentDir)
}

// EventsDir returns the directory for daily event log files.
func EventsDir() string {
	return filepath.Join(StateDir(), eventsDir)
}

// EnsureDirs creates the state directories if they don't exist.
func EnsureDirs() error {
	for _, dir := range []string{CurrentDir(), EventsDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create state dir %s: %w", dir, err)
		}
	}
	return nil
}

// StateFilePath returns the path to the current-state file for a given working directory.
func StateFilePath(cwd string) string {
	hash := util.DirHash(cwd)
	return filepath.Join(CurrentDir(), hash+".json")
}

// WriteState atomically writes a HookState to its per-directory state file.
// It writes to a temp file first, then renames into place.
func WriteState(state *model.HookState) error {
	if err := EnsureDirs(); err != nil {
		return err
	}

	target := StateFilePath(state.Cwd)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	// Write to temp file in the same directory (same filesystem for atomic rename).
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".ttyrant-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename state file: %w", err)
	}

	return nil
}

// RemoveState removes the state file for a given working directory.
func RemoveState(cwd string) error {
	path := StateFilePath(cwd)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
