package vectorstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// BranchMetadata tracks indexing state per branch
// This allows incremental re-indexing when commits change
type BranchMetadata struct {
	RepoName  string    `json:"repo_name"`
	Branch    string    `json:"branch"`
	CommitSHA string    `json:"commit_sha"`
	IndexedAt time.Time `json:"indexed_at"`
	FileCount int       `json:"file_count"`
}

// GetMetadataPath returns path to metadata file for repo+branch
// Example: .mesh/my-repo/main/metadata.json
func GetMetadataPath(repoName, branch string) string {
	// Sanitize branch name for filesystem (replace / with -)
	safeBranch := SanitizeBranchName(branch)
	return filepath.Join(".mesh", repoName, safeBranch, "metadata.json")
}

// LoadMetadata loads the metadata for a repo+branch combination
// Returns nil if no metadata exists yet (first time indexing)
func LoadMetadata(repoName, branch string) (*BranchMetadata, error) {
	path := GetMetadataPath(repoName, branch)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No metadata yet - first time indexing
		}
		return nil, err
	}

	var meta BranchMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// SaveMetadata saves the metadata for a repo+branch combination
func SaveMetadata(meta *BranchMetadata) error {
	path := GetMetadataPath(meta.RepoName, meta.Branch)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetKnownBranches returns a list of branches that have been indexed
// by reading the .mesh/{repo}/ directory structure
func GetKnownBranches(repoName string) ([]string, error) {
	repoMetaPath := filepath.Join(".mesh", repoName)

	// Check if repo metadata directory exists
	if _, err := os.Stat(repoMetaPath); os.IsNotExist(err) {
		return []string{}, nil // No branches indexed yet
	}

	// Read branch directories
	entries, err := os.ReadDir(repoMetaPath)
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Read metadata file to get the original branch name (with slashes)
			// Directory names are sanitized, but we need the original name for git commands
			metaPath := filepath.Join(repoMetaPath, entry.Name(), "metadata.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				// Skip if metadata file doesn't exist or can't be read
				continue
			}

			var meta BranchMetadata
			if err := json.Unmarshal(data, &meta); err != nil {
				// Skip if metadata is corrupted
				continue
			}

			// Use the original branch name from metadata
			branches = append(branches, meta.Branch)
		}
	}

	return branches, nil
}

// NeedsReindexing checks if a repository needs to be re-indexed
// Returns true if:
// - No metadata exists (never indexed)
// - Current commit differs from last indexed commit
func NeedsReindexing(repoPath, repoName, branch string) (bool, string, error) {
	// Get current commit for this branch
	currentCommit, err := GetBranchCommit(repoPath, branch)
	if err != nil {
		return false, "", err
	}

	// Load metadata
	meta, err := LoadMetadata(repoName, branch)
	if err != nil {
		return false, "", err
	}

	// No metadata = never indexed
	if meta == nil {
		return true, currentCommit, nil
	}

	// Check if commit changed
	return meta.CommitSHA != currentCommit, currentCommit, nil
}
