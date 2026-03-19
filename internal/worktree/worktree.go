package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// BareRepo represents a bare git repository set up for worktrees.
type BareRepo struct {
	Name string
	Path string
}

// Project represents any project directory.
type Project struct {
	Name   string
	Path   string
	IsBare bool
}

// Worktree represents a single git worktree within a bare repo.
type Worktree struct {
	Path   string
	Branch string
	Head   string
	IsBare bool
}

// ProjectsDir returns the fixed projects directory.
func ProjectsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Projects")
}

// ConfigDir returns the user config directory.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// ScanAllProjects finds all project directories under ~/Projects and ~/.config.
func ScanAllProjects() ([]Project, error) {
	dirs := []string{ProjectsDir(), ConfigDir()}
	var projects []Project
	seen := make(map[string]bool)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if seen[name] {
				continue
			}
			seen[name] = true

			path := filepath.Join(dir, name)
			barePath := filepath.Join(path, ".bare")
			info, statErr := os.Stat(barePath)
			isBare := statErr == nil && info.IsDir()

			projects = append(projects, Project{
				Name:   name,
				Path:   path,
				IsBare: isBare,
			})
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

// ScanRepos finds all bare-repo directories under projectsDir.
func ScanRepos(projectsDir string) ([]BareRepo, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var repos []BareRepo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		barePath := filepath.Join(projectsDir, e.Name(), ".bare")
		info, err := os.Stat(barePath)
		if err == nil && info.IsDir() {
			repos = append(repos, BareRepo{
				Name: e.Name(),
				Path: filepath.Join(projectsDir, e.Name()),
			})
		}
	}
	return repos, nil
}

// ListWorktrees returns all non-bare worktrees for a repo.
func ListWorktrees(repoPath string) ([]Worktree, error) {
	out, err := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []Worktree
	var current *Worktree

	for line := range strings.SplitSeq(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current != nil && !current.IsBare {
				worktrees = append(worktrees, *current)
			}
			current = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			if current != nil {
				h := strings.TrimPrefix(line, "HEAD ")
				if len(h) > 7 {
					h = h[:7]
				}
				current.Head = h
			}
		case strings.HasPrefix(line, "branch "):
			if current != nil {
				current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			}
		case line == "bare":
			if current != nil {
				current.IsBare = true
			}
		case line == "detached":
			if current != nil && current.Branch == "" {
				current.Branch = "(detached)"
			}
		}
	}
	if current != nil && !current.IsBare {
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

// ListRemoteBranches returns remote branch names with origin/ prefix stripped.
func ListRemoteBranches(repoPath string) ([]string, error) {
	out, err := exec.Command("git", "-C", repoPath, "branch", "-r", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, fmt.Errorf("git branch -r: %w", err)
	}

	var branches []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		branch := strings.TrimPrefix(line, "origin/")
		if branch == "HEAD" {
			continue
		}
		if !seen[branch] {
			seen[branch] = true
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// CreateWorktree creates a worktree for the given branch.
// If the branch doesn't exist remotely, a new local branch is created.
// Returns the worktree path.
func CreateWorktree(repoPath, branch string) (string, error) {
	wtPath := filepath.Join(repoPath, branch)

	if _, err := os.Stat(wtPath); err == nil {
		return wtPath, nil
	}

	// Try checking out an existing branch.
	if err := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch).Run(); err == nil {
		return wtPath, nil
	}

	// Fall back to creating a new branch.
	if err := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branch, wtPath).Run(); err != nil {
		return "", fmt.Errorf("create worktree for branch %q: %w", branch, err)
	}
	return wtPath, nil
}

// CreateWorktreeCmd returns the worktree path and an exec.Cmd that creates it
// with output visible to the user. Returns ("", nil) if the worktree already exists.
func CreateWorktreeCmd(repoPath, branch string) (string, *exec.Cmd) {
	wtPath := filepath.Join(repoPath, branch)

	if _, err := os.Stat(wtPath); err == nil {
		return wtPath, nil
	}

	script := fmt.Sprintf(
		`git -C %q worktree add %q %q 2>&1 || git -C %q worktree add -b %q %q 2>&1`,
		repoPath, wtPath, branch,
		repoPath, branch, wtPath,
	)
	return wtPath, exec.Command("sh", "-c", script)
}

// RemoveWorktree removes a git worktree.
func RemoveWorktree(repoPath, worktreePath string) error {
	if err := exec.Command("git", "-C", repoPath, "worktree", "remove", worktreePath).Run(); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

// RemoveWorktreeCmd returns an exec.Cmd that removes a worktree with output visible.
func RemoveWorktreeCmd(repoPath, worktreePath string) *exec.Cmd {
	return exec.Command("git", "-C", repoPath, "worktree", "remove", worktreePath)
}

// CloneBareCmd returns an exec.Cmd that clones a bare repo with output visible,
// and performs the remaining setup steps.
func CloneBareCmd(url, projectsDir string) (*exec.Cmd, error) {
	name := filepath.Base(url)
	name = strings.TrimSuffix(name, ".git")

	repoPath := filepath.Join(projectsDir, name)
	barePath := filepath.Join(repoPath, ".bare")
	gitFile := filepath.Join(repoPath, ".git")

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	script := fmt.Sprintf(`set -e
git clone --bare %q %q
echo "gitdir: .bare" > %q
git -C %q config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"
echo "Fetching branches..."
git -C %q fetch origin
MAIN=$(git -C %q symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's|refs/remotes/origin/||' || echo "main")
echo "Creating worktree $MAIN..."
git -C %q worktree add %q/"$MAIN" "$MAIN"
echo "Done."`,
		url, barePath,
		gitFile,
		repoPath,
		repoPath,
		repoPath,
		repoPath, repoPath,
	)
	return exec.Command("sh", "-c", script), nil
}

// CloneBare clones a repository as a bare repo set up for worktrees.
// Mirrors the git-clone-bare shell function.
func CloneBare(url, projectsDir string) error {
	name := filepath.Base(url)
	name = strings.TrimSuffix(name, ".git")

	repoPath := filepath.Join(projectsDir, name)
	barePath := filepath.Join(repoPath, ".bare")
	gitFile := filepath.Join(repoPath, ".git")

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := exec.Command("git", "clone", "--bare", url, barePath).Run(); err != nil {
		return fmt.Errorf("git clone --bare: %w", err)
	}

	if err := os.WriteFile(gitFile, []byte("gitdir: .bare\n"), 0o644); err != nil {
		return fmt.Errorf("write .git: %w", err)
	}

	if err := exec.Command("git", "-C", repoPath, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*").Run(); err != nil {
		return fmt.Errorf("git config fetch: %w", err)
	}

	if err := exec.Command("git", "-C", repoPath, "fetch", "origin").Run(); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	mainBranch := "main"
	if out, err := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
		if ref != "" {
			mainBranch = ref
		}
	}

	wtPath := filepath.Join(repoPath, mainBranch)
	if err := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, mainBranch).Run(); err != nil {
		return fmt.Errorf("create initial worktree: %w", err)
	}

	return nil
}

var sanitizeRe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// SanitizeName converts a string into a valid tmux session name component.
func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	return sanitizeRe.ReplaceAllString(name, "_")
}

// SessionName returns the expected tmux session name for a repo+branch combo.
func SessionName(repoName, branch string) string {
	return SanitizeName(repoName + "-" + branch)
}
