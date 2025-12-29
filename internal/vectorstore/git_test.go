package vectorstore

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSanitizeBranchName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"main", "main"},
		{"develop", "develop"},
		{"feature/auth-v2", "feature-auth-v2"},
		{"bugfix/issue-123", "bugfix-issue-123"},
		{"hotfix/prod\\fix", "hotfix-prod-fix"},
		{"release:v1.2.3", "release-v1.2.3"},
		{"feat/api/v2/users", "feat-api-v2-users"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := SanitizeBranchName(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeBranchName(%s): expected '%s', got '%s'", tc.input, tc.expected, result)
			}
		})
	}
}

func TestSanitizeBranchName_NoSpecialChars(t *testing.T) {
	// Branches without special chars should be unchanged
	branches := []string{"main", "develop", "production", "staging"}

	for _, branch := range branches {
		result := SanitizeBranchName(branch)
		if result != branch {
			t.Errorf("SanitizeBranchName(%s) should be unchanged, got '%s'", branch, result)
		}
	}
}

func TestIsGitRepo_WithActualRepo(t *testing.T) {
	// Create a temp git repo for testing
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", tmpDir)
	err := cmd.Run()
	if err != nil {
		t.Skipf("Skipping test: git init failed (git may not be available): %v", err)
	}

	// Test IsGitRepo
	isRepo := IsGitRepo(tmpDir)
	if !isRepo {
		t.Error("IsGitRepo should return true for initialized git repository")
	}
}

func TestIsGitRepo_NonGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't initialize git, just create an empty directory
	isRepo := IsGitRepo(tmpDir)
	if isRepo {
		t.Error("IsGitRepo should return false for non-git directory")
	}
}

func TestIsGitRepo_NonexistentPath(t *testing.T) {
	isRepo := IsGitRepo("/nonexistent/path/to/nowhere")
	if isRepo {
		t.Error("IsGitRepo should return false for nonexistent path")
	}
}

func TestGetCurrentBranch_WithActualRepo(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", "-b", "main", tmpDir)
	err := cmd.Run()
	if err != nil {
		t.Skipf("Skipping test: git init failed: %v", err)
	}

	// Get current branch
	branch, err := GetCurrentBranch(tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}

	if branch != "main" && branch != "master" {
		t.Errorf("Expected branch 'main' or 'master', got '%s'", branch)
	}
}

func TestGetHeadCommit_WithActualRepo(t *testing.T) {
	// Create a temp git repo with a commit
	tmpDir := t.TempDir()

	// Initialize git repo
	if exec.Command("git", "init", tmpDir).Run() != nil {
		t.Skip("Skipping test: git not available")
	}

	// Configure git user (required for commit)
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create a file and commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	err := exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()
	if err != nil {
		t.Skipf("Skipping test: commit failed: %v", err)
	}

	// Get HEAD commit
	commit, err := GetHeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("GetHeadCommit failed: %v", err)
	}

	// Commit SHA should be 40 characters (hex)
	if len(commit) != 40 {
		t.Errorf("Expected commit SHA length 40, got %d: %s", len(commit), commit)
	}
}

func TestGetCurrentBranch_NonGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetCurrentBranch(tmpDir)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestGetHeadCommit_NonGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetHeadCommit(tmpDir)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestGetAllBranches_NonGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetAllBranches(tmpDir)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestGetBranchCommit_NonGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetBranchCommit(tmpDir, "main")
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestGetChangedFilesSince_NonGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetChangedFilesSince(tmpDir, "abc123")
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}
