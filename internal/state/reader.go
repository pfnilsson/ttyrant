package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pfnilsson/ttyrant/internal/model"
)

// ReadStateFile reads the current-state file for a given working directory.
// Returns nil, nil if the file doesn't exist.
func ReadStateFile(cwd string) (*model.HookState, error) {
	path := StateFilePath(cwd)
	return readStateFileAt(path)
}

// ReadAllStates reads all current-state JSON files from the state directory.
func ReadAllStates() ([]model.HookState, error) {
	dir := CurrentDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var states []model.HookState
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		s, err := readStateFileAt(path)
		if err != nil {
			// Skip corrupt files.
			continue
		}
		if s != nil {
			states = append(states, *s)
		}
	}

	return states, nil
}

func readStateFileAt(path string) (*model.HookState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var s model.HookState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
