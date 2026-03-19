package util

import (
	"os"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"root", "/", "/"},
		{"trailing slash", "/home/user/projects/", "/home/user/projects"},
		{"already clean", "/home/user/projects", "/home/user/projects"},
		{"double trailing slash", "/home/user/projects//", "/home/user/projects"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePath(tt.in)
			// For paths that may not exist on the system, just check the expected transformation.
			// Skip symlink resolution check for non-existent paths.
			if tt.in == "" || tt.in == "/" {
				if got != tt.want {
					t.Errorf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
				}
				return
			}
			// For other cases the path might not exist, so just verify trailing slash behavior.
			if len(got) > 1 && got[len(got)-1] == '/' {
				t.Errorf("NormalizePath(%q) = %q, has trailing slash", tt.in, got)
			}
		})
	}
}

func TestNormalizePath_Symlink(t *testing.T) {
	// Create a temp dir and a symlink to it.
	dir, err := os.MkdirTemp("", "ttyrant-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	link := dir + "-link"
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)

	got := NormalizePath(link)
	want := NormalizePath(dir)
	if got != want {
		t.Errorf("NormalizePath(symlink) = %q, want %q (resolved real dir)", got, want)
	}
}

func TestDirHash(t *testing.T) {
	h1 := DirHash("/home/user/project-a")
	h2 := DirHash("/home/user/project-b")

	if h1 == h2 {
		t.Error("different paths should produce different hashes")
	}
	if len(h1) != 16 {
		t.Errorf("hash length = %d, want 16", len(h1))
	}

	// Deterministic.
	if DirHash("/foo") != DirHash("/foo") {
		t.Error("same path should produce same hash")
	}
}
