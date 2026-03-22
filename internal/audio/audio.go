package audio

import (
	_ "embed"
	"fmt"
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
	if err := os.WriteFile(tmpFile, kachingWav, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "ttyrant: warning: could not write audio file: %v\n", err)
		playerCmd = "" // disable audio
	}
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

// Cleanup removes the temporary audio file. Call on application exit.
func Cleanup() {
	if tmpFile != "" {
		os.Remove(tmpFile)
	}
}
