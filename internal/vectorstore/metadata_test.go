package vectorstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetMetadataPath(t *testing.T) {
	path := GetMetadataPath("test-repo", "main")
	expected := filepath.Join(".mesh", "test-repo", "main", "metadata.json")

	if path != expected {
		t.Errorf("Expected path '%s', got '%s'", expected, path)
	}
}

func TestGetMetadataPath_WithSlashInBranch(t *testing.T) {
	// Branches with slashes should be sanitized
	path := GetMetadataPath("test-repo", "feature/new-api")

	// Should contain sanitized branch name
	if !contains(path, "test-repo") {
		t.Error("Path should contain repo name")
	}

	if !contains(path, "metadata.json") {
		t.Error("Path should end with metadata.json")
	}
}

func TestSaveAndLoadMetadata(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	meta := &BranchMetadata{
		RepoName:  "test-repo",
		Branch:    "main",
		CommitSHA: "abc123def456",
		IndexedAt: time.Now(),
		FileCount: 42,
	}

	// Save metadata
	err := SaveMetadata(meta)
	if err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	// Verify file was created
	path := GetMetadataPath("test-repo", "main")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Metadata file was not created")
	}

	// Load metadata
	loaded, err := LoadMetadata("test-repo", "main")
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("LoadMetadata returned nil")
	}

	// Verify fields match
	if loaded.RepoName != meta.RepoName {
		t.Errorf("Expected RepoName '%s', got '%s'", meta.RepoName, loaded.RepoName)
	}

	if loaded.Branch != meta.Branch {
		t.Errorf("Expected Branch '%s', got '%s'", meta.Branch, loaded.Branch)
	}

	if loaded.CommitSHA != meta.CommitSHA {
		t.Errorf("Expected CommitSHA '%s', got '%s'", meta.CommitSHA, loaded.CommitSHA)
	}

	if loaded.FileCount != meta.FileCount {
		t.Errorf("Expected FileCount %d, got %d", meta.FileCount, loaded.FileCount)
	}
}

func TestLoadMetadata_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Load metadata that doesn't exist
	loaded, err := LoadMetadata("nonexistent-repo", "main")
	if err != nil {
		t.Errorf("LoadMetadata should return nil without error for nonexistent file, got error: %v", err)
	}

	if loaded != nil {
		t.Error("LoadMetadata should return nil for nonexistent file")
	}
}

func TestLoadMetadata_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create invalid JSON file
	path := GetMetadataPath("test-repo", "main")
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	err = os.WriteFile(path, []byte("invalid json content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	// Load should fail
	_, err = LoadMetadata("test-repo", "main")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetKnownBranches_NoMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	branches, err := GetKnownBranches("nonexistent-repo")
	if err != nil {
		t.Errorf("GetKnownBranches should not error for nonexistent repo, got: %v", err)
	}

	if len(branches) != 0 {
		t.Errorf("Expected 0 branches, got %d", len(branches))
	}
}

func TestGetKnownBranches_MultipleBranches(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Save metadata for multiple branches
	branches := []string{"main", "develop", "feature-a"}
	for _, branch := range branches {
		meta := &BranchMetadata{
			RepoName:  "test-repo",
			Branch:    branch,
			CommitSHA: "abc123",
			IndexedAt: time.Now(),
			FileCount: 10,
		}
		err := SaveMetadata(meta)
		if err != nil {
			t.Fatalf("SaveMetadata failed for branch %s: %v", branch, err)
		}
	}

	// Get known branches
	knownBranches, err := GetKnownBranches("test-repo")
	if err != nil {
		t.Fatalf("GetKnownBranches failed: %v", err)
	}

	if len(knownBranches) != 3 {
		t.Errorf("Expected 3 branches, got %d", len(knownBranches))
	}

	// Verify all branches are present
	branchSet := make(map[string]bool)
	for _, b := range knownBranches {
		branchSet[b] = true
	}

	for _, expected := range branches {
		if !branchSet[expected] {
			t.Errorf("Expected branch '%s' not found in results", expected)
		}
	}
}

func TestBranchMetadata_JSONRoundtrip(t *testing.T) {
	original := &BranchMetadata{
		RepoName:  "my-repo",
		Branch:    "develop",
		CommitSHA: "deadbeef123",
		IndexedAt: time.Now().Truncate(time.Second), // Truncate for comparison
		FileCount: 100,
	}

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Save
	err := SaveMetadata(original)
	if err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	// Load
	loaded, err := LoadMetadata("my-repo", "develop")
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}

	// Compare all fields
	if loaded.RepoName != original.RepoName {
		t.Errorf("RepoName mismatch: expected '%s', got '%s'", original.RepoName, loaded.RepoName)
	}

	if loaded.Branch != original.Branch {
		t.Errorf("Branch mismatch: expected '%s', got '%s'", original.Branch, loaded.Branch)
	}

	if loaded.CommitSHA != original.CommitSHA {
		t.Errorf("CommitSHA mismatch: expected '%s', got '%s'", original.CommitSHA, loaded.CommitSHA)
	}

	if loaded.FileCount != original.FileCount {
		t.Errorf("FileCount mismatch: expected %d, got %d", original.FileCount, loaded.FileCount)
	}

	// IndexedAt might have slight differences due to JSON marshaling, check it's close
	timeDiff := loaded.IndexedAt.Sub(original.IndexedAt)
	if timeDiff < -time.Second || timeDiff > time.Second {
		t.Errorf("IndexedAt differs too much: original %v, loaded %v", original.IndexedAt, loaded.IndexedAt)
	}
}

// Helper function
func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
