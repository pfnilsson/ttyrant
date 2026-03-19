package audio

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed kaching.wav
var kachingWav []byte

var (
	setupOnce sync.Once
	playerCmd string
	tmpFile   string
)

func setup() {
	for _, name := range []string{"paplay", "aplay", "afplay"} {
		if _, err := exec.LookPath(name); err == nil {
			playerCmd = name
			break
		}
	}
	if playerCmd == "" {
		return
	}

	tmpFile = filepath.Join(os.TempDir(), "ttyrant-kaching.wav")
	_ = os.WriteFile(tmpFile, kachingWav, 0644)
}

// Play plays the embedded kaching sound asynchronously.
func Play() {
	setupOnce.Do(setup)
	if playerCmd == "" {
		return
	}

	go func() {
		cmd := exec.Command(playerCmd, tmpFile)
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}()
}
