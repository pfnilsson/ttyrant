package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initBareRepo creates a bare repo structure in tmpDir suitable for worktree tests.
// Returns the repo root path (parent of .bare).
func initBareRepo(t *testing.T, tmpDir, name string) string {
	t.Helper()

	repoPath := filepath.Join(tmpDir, name)
	barePath := filepath.Join(repoPath, ".bare")

	// Create an upstream repo to clone from.
	upstream := filepath.Join(tmpDir, name+"-upstream")
	gitEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test",
	)

	for _, cmd := range [][]string{
		{"git", "init", upstream},
		{"git", "-C", upstream, "commit", "--allow-empty", "-m", "initial"},
		{"git", "-C", upstream, "branch", "-M", "main"},
		{"git", "clone", "--bare", upstream, barePath},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Env = gitEnv
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", cmd, out)
		}
	}

	if err := os.WriteFile(filepath.Join(repoPath, ".git"), []byte("gitdir: .bare\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		c := exec.Command(args[0], args[1:]...)
		c.Env = gitEnv
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	run("git", "-C", repoPath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	run("git", "-C", repoPath, "fetch", "origin")
	run("git", "-C", repoPath, "worktree", "add", filepath.Join(repoPath, "main"), "main")

	return repoPath
}

func TestScanRepos(t *testing.T) {
	tmp := t.TempDir()

	// Create a bare repo and a non-bare directory.
	initBareRepo(t, tmp, "myrepo")
	os.MkdirAll(filepath.Join(tmp, "notbare"), 0o755)

	repos, err := ScanRepos(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Name != "myrepo" {
		t.Errorf("expected name=myrepo, got %q", repos[0].Name)
	}
	if repos[0].Path != filepath.Join(tmp, "myrepo") {
		t.Errorf("expected path=%s, got %s", filepath.Join(tmp, "myrepo"), repos[0].Path)
	}
}

func TestScanRepos_Empty(t *testing.T) {
	tmp := t.TempDir()
	repos, err := ScanRepos(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}
}

func TestScanRepos_MissingDir(t *testing.T) {
	_, err := ScanRepos("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestListWorktrees(t *testing.T) {
	tmp := t.TempDir()
	repoPath := initBareRepo(t, tmp, "testrepo")

	wts, err := ListWorktrees(repoPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if wts[0].Branch != "main" {
		t.Errorf("expected branch=main, got %q", wts[0].Branch)
	}
	if wts[0].Head == "" {
		t.Error("expected non-empty HEAD")
	}
	if wts[0].IsBare {
		t.Error("expected IsBare=false for the main worktree")
	}
}

func TestCreateWorktree_NewBranch(t *testing.T) {
	tmp := t.TempDir()
	repoPath := initBareRepo(t, tmp, "testrepo")

	wtPath, err := CreateWorktree(repoPath, "feat-test")
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(repoPath, "feat-test")
	if wtPath != expected {
		t.Errorf("expected path=%s, got %s", expected, wtPath)
	}

	info, err := os.Stat(wtPath)
	if err != nil || !info.IsDir() {
		t.Fatal("worktree directory does not exist")
	}

	// Verify it shows up in ListWorktrees.
	wts, err := ListWorktrees(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, wt := range wts {
		if wt.Branch == "feat-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new worktree not found in ListWorktrees")
	}
}

func TestCreateWorktree_ExistingBranch(t *testing.T) {
	tmp := t.TempDir()
	repoPath := initBareRepo(t, tmp, "testrepo")

	// Create a branch in the upstream via the bare repo.
	c := exec.Command("git", "-C", filepath.Join(repoPath, "main"), "branch", "existing-branch")
	c.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
	)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("create branch: %s", out)
	}

	wtPath, err := CreateWorktree(repoPath, "existing-branch")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(wtPath); err != nil {
		t.Fatal("worktree directory does not exist")
	}
}

func TestCreateWorktree_AlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	repoPath := initBareRepo(t, tmp, "testrepo")

	// main worktree already exists from init.
	wtPath, err := CreateWorktree(repoPath, "main")
	if err != nil {
		t.Fatal(err)
	}
	if wtPath != filepath.Join(repoPath, "main") {
		t.Errorf("unexpected path: %s", wtPath)
	}
}

func TestRemoveWorktree(t *testing.T) {
	tmp := t.TempDir()
	repoPath := initBareRepo(t, tmp, "testrepo")

	// Create a worktree to remove.
	wtPath, err := CreateWorktree(repoPath, "to-remove")
	if err != nil {
		t.Fatal(err)
	}

	if err := RemoveWorktree(repoPath, wtPath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}

	// Verify it's gone from ListWorktrees.
	wts, err := ListWorktrees(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range wts {
		if wt.Branch == "to-remove" {
			t.Error("removed worktree still appears in list")
		}
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"my.repo", "my_repo"},
		{"feat/branch", "feat_branch"},
		{"a.b/c@d", "a_b_c_d"},
		{"ok-name_here", "ok-name_here"},
		{"spaces here", "spaces_here"},
	}
	for _, tt := range tests {
		got := SanitizeName(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSessionName(t *testing.T) {
	tests := []struct {
		repo   string
		branch string
		want   string
	}{
		{"ttyrant", "main", "ttyrant-main"},
		{"my.repo", "feat/thing", "my_repo-feat_thing"},
		{"repo", "fix-123", "repo-fix-123"},
	}
	for _, tt := range tests {
		got := SessionName(tt.repo, tt.branch)
		if got != tt.want {
			t.Errorf("SessionName(%q, %q) = %q, want %q", tt.repo, tt.branch, got, tt.want)
		}
	}
}
