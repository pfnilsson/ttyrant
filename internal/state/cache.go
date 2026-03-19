package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pfnilsson/ttyrant/internal/model"
)

const cacheFile = "view-cache.json"

// CachePath returns the path to the view cache file.
func CachePath() string {
	return filepath.Join(StateDir(), cacheFile)
}

// WriteCache persists the current session rows for fast startup.
func WriteCache(rows []model.SessionRow) error {
	if err := EnsureDirs(); err != nil {
		return err
	}

	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}

	return os.WriteFile(CachePath(), data, 0o644)
}

// ReadCache loads cached session rows and recomputes IdleFor.
func ReadCache() []model.SessionRow {
	data, err := os.ReadFile(CachePath())
	if err != nil {
		return nil
	}

	var rows []model.SessionRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}

	now := time.Now()
	for i := range rows {
		if !rows[i].LastEventAt.IsZero() {
			rows[i].IdleFor = now.Sub(rows[i].LastEventAt)
		}
	}

	return rows
}
