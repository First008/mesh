package vectorstore

import (
	"fmt"
	"os/exec"
	"strings"
)

// Git command wrappers for branch-aware indexing

// GetCurrentBranch returns the current git branch name for a repository
func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetHeadCommit returns the current HEAD commit SHA for a repository
func GetHeadCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get HEAD commit: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetChangedFilesSince returns a list of files that changed between fromCommit and HEAD
// This is used for incremental indexing after git pull
func GetChangedFilesSince(repoPath, fromCommit string) ([]string, error) {
	// Get files changed between fromCommit and HEAD
	cmd := exec.Command("git", "-C", repoPath, "diff", "--name-only", fromCommit, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get changed files: %w", err)
	}

	if len(out) == 0 {
		return []string{}, nil
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	return files, nil
}

// IsGitRepo checks if the given path is a git repository
func IsGitRepo(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// GetAllBranches returns a list of all local branches in a repository
func GetAllBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get branches: %w", err)
	}

	if len(out) == 0 {
		return []string{}, nil
	}

	branches := strings.Split(strings.TrimSpace(string(out)), "\n")
	return branches, nil
}

// GetBranchCommit returns the HEAD commit SHA for a specific branch
func GetBranchCommit(repoPath, branch string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", branch)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get branch commit: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SanitizeBranchName converts a branch name to a filesystem-safe string
// Example: "feature/auth-v2" -> "feature-auth-v2"
func SanitizeBranchName(branch string) string {
	// Replace slashes and other problematic characters
	sanitized := strings.ReplaceAll(branch, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	return sanitized
}
