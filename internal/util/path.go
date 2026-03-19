package util

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath returns a canonical absolute path:
// - resolves to absolute
// - resolves symlinks when possible
// - trims trailing slash (except root "/")
// - preserves case
func NormalizePath(p string) string {
	if p == "" {
		return ""
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}

	// Resolve symlinks if possible.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		resolved = abs
	}

	// Trim trailing slash except root.
	if resolved != "/" {
		resolved = strings.TrimRight(resolved, string(os.PathSeparator))
	}

	return resolved
}

// DirHash returns a truncated SHA256 hex string for a normalized directory path.
// Used as the filename for per-directory state files.
func DirHash(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars
}
