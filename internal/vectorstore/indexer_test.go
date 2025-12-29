package vectorstore

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

// mockVectorStore for indexer testing (avoid import cycle)
type mockStore struct {
	indexed   map[string]string
	indexCnt  int
	deleteCnt int
	mu        sync.Mutex
}

func newMockStore() *mockStore {
	return &mockStore{
		indexed: make(map[string]string),
	}
}

func (m *mockStore) IndexFile(ctx context.Context, filePath, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexed[filePath] = content
	m.indexCnt++
	return nil
}

func (m *mockStore) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return []SearchResult{}, nil
}

func (m *mockStore) DeleteFile(ctx context.Context, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.indexed, filePath)
	m.deleteCnt++
	return nil
}

func (m *mockStore) DeleteCollection(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexed = make(map[string]string)
	return nil
}

func (m *mockStore) GetStats(ctx context.Context) (*Stats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &Stats{TotalVectors: int64(len(m.indexed))}, nil
}

func (m *mockStore) SearchWithAggregation(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// Delegate to Search for test mocks
	return m.Search(ctx, query, limit)
}

func (m *mockStore) Close() error {
	return nil
}

func TestNewIndexer(t *testing.T) {
	store := newMockStore()
	indexer := NewIndexer(store, "/tmp/test-repo", testLogger())

	if indexer == nil {
		t.Fatal("Expected indexer, got nil")
	}

	if indexer.store == nil {
		t.Error("Indexer store should not be nil")
	}

	if indexer.repoPath != "/tmp/test-repo" {
		t.Errorf("Expected repoPath '/tmp/test-repo', got '%s'", indexer.repoPath)
	}

	if indexer.fileHashes == nil {
		t.Error("Indexer fileHashes map should be initialized")
	}
}

func TestNewIndexerWithBranch(t *testing.T) {
	store := newMockStore()
	indexer := NewIndexerWithBranch(store, "/tmp/test-repo", "my-repo", "develop", testLogger())

	if indexer == nil {
		t.Fatal("Expected indexer, got nil")
	}

	if indexer.repoName != "my-repo" {
		t.Errorf("Expected repoName 'my-repo', got '%s'", indexer.repoName)
	}

	if indexer.branch != "develop" {
		t.Errorf("Expected branch 'develop', got '%s'", indexer.branch)
	}

	if indexer.repoPath != "/tmp/test-repo" {
		t.Errorf("Expected repoPath '/tmp/test-repo', got '%s'", indexer.repoPath)
	}
}

func TestIndexRepository_WithMockFiles(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create some Go files
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "utils.go"), []byte("package utils\nfunc Helper() {}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a non-code file (should be skipped)
	err = os.WriteFile(filepath.Join(tmpDir, "README.txt"), []byte("This is a readme"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	// Index the repository
	err = indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed: %v", err)
	}

	// Verify at least the .go files were indexed
	if store.indexCnt < 2 {
		t.Errorf("Expected at least 2 files indexed (Go files), got %d", store.indexCnt)
	}

	// Check specific files
	if _, ok := store.indexed["main.go"]; !ok {
		t.Error("main.go should be indexed")
	}

	if _, ok := store.indexed["utils.go"]; !ok {
		t.Error("utils.go should be indexed")
	}
}

func TestIndexRepository_SkipsNodeModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create node_modules directory
	nodeModules := filepath.Join(tmpDir, "node_modules")
	err := os.MkdirAll(nodeModules, 0755)
	if err != nil {
		t.Fatalf("Failed to create node_modules: %v", err)
	}

	// Create a file in node_modules (should be skipped)
	err = os.WriteFile(filepath.Join(nodeModules, "lib.js"), []byte("module.exports = {}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a file in root (should be indexed)
	err = os.WriteFile(filepath.Join(tmpDir, "index.js"), []byte("console.log('test')"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	err = indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed: %v", err)
	}

	// Only root file should be indexed
	if _, ok := store.indexed["node_modules/lib.js"]; ok {
		t.Error("node_modules files should be skipped")
	}

	if _, ok := store.indexed["index.js"]; !ok {
		t.Error("root index.js should be indexed")
	}
}

func TestIndexRepository_SkipsGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	err := os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git: %v", err)
	}

	// Create a file in .git (should be skipped)
	err = os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create git config: %v", err)
	}

	// Create a file in root
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	err = indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed: %v", err)
	}

	// .git files should be skipped
	for path := range store.indexed {
		if len(path) > 4 && path[:4] == ".git" {
			t.Errorf(".git directory should be skipped, but found: %s", path)
		}
	}
}

func TestIndexRepository_SkipsLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a large file (>500KB)
	largeContent := make([]byte, 600000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}

	err := os.WriteFile(filepath.Join(tmpDir, "large.go"), largeContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Create a small file (should be indexed)
	err = os.WriteFile(filepath.Join(tmpDir, "small.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	err = indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed: %v", err)
	}

	// Large file should be skipped
	if _, ok := store.indexed["large.go"]; ok {
		t.Error("Large file (>500KB) should be skipped")
	}

	// Small file should be indexed
	if _, ok := store.indexed["small.go"]; !ok {
		t.Error("Small file should be indexed")
	}
}

func TestIndexStats_ThreadSafety(t *testing.T) {
	stats := &IndexStats{}

	// Concurrently increment counters
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			stats.incIndexed()
			stats.incSkipped()
			stats.incErrors()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	indexed, skipped, errors := stats.get()

	// Each counter should be 100
	if indexed != 100 {
		t.Errorf("Expected 100 indexed, got %d", indexed)
	}

	if skipped != 100 {
		t.Errorf("Expected 100 skipped, got %d", skipped)
	}

	if errors != 100 {
		t.Errorf("Expected 100 errors, got %d", errors)
	}
}

func TestIndexStats_Get(t *testing.T) {
	stats := &IndexStats{}

	// Initial state
	indexed, skipped, errors := stats.get()
	if indexed != 0 || skipped != 0 || errors != 0 {
		t.Errorf("Expected all zeros initially, got indexed=%d, skipped=%d, errors=%d",
			indexed, skipped, errors)
	}

	// Increment some
	stats.incIndexed()
	stats.incIndexed()
	stats.incSkipped()
	stats.incErrors()

	indexed, skipped, errors = stats.get()
	if indexed != 2 {
		t.Errorf("Expected 2 indexed, got %d", indexed)
	}

	if skipped != 1 {
		t.Errorf("Expected 1 skipped, got %d", skipped)
	}

	if errors != 1 {
		t.Errorf("Expected 1 error, got %d", errors)
	}
}

func TestIndexJob_Structure(t *testing.T) {
	job := IndexJob{
		RelPath: "src/main.go",
		Content: "package main\n\nfunc main() {}",
	}

	if job.RelPath != "src/main.go" {
		t.Errorf("Expected RelPath 'src/main.go', got '%s'", job.RelPath)
	}

	if len(job.Content) == 0 {
		t.Error("Content should not be empty")
	}
}

func TestIndexRepository_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create any files

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	err := indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed on empty dir: %v", err)
	}

	// No files should be indexed
	if store.indexCnt != 0 {
		t.Errorf("Expected 0 files indexed in empty directory, got %d", store.indexCnt)
	}
}

func TestIndexRepository_NonexistentDirectory(t *testing.T) {
	store := newMockStore()
	indexer := NewIndexer(store, "/nonexistent/path/that/does/not/exist", testLogger())

	err := indexer.IndexRepository(context.Background())
	// filepath.Walk will return an error for nonexistent path
	// If implementation changes to handle this gracefully, that's also acceptable
	if err == nil {
		// Check that no files were indexed
		if store.indexCnt > 0 {
			t.Error("Should not index any files from nonexistent directory")
		}
	}
}

func TestIndexRepository_MultipleFileTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create various file types
	testFiles := map[string]string{
		"main.go":       "package main",
		"utils.ts":      "export function test() {}",
		"helper.py":     "def helper():\n    pass",
		"config.yaml":   "key: value",
		"README.md":     "# README",
		"data.json":     `{"key": "value"}`,
		"script.sh":     "#!/bin/bash\necho hello",
		"ignored.txt":   "should be skipped",
		"binary.bin":    "binary content",
	}

	for filename, content := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", filename, err)
		}
	}

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	err := indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed: %v", err)
	}

	// Verify code files were indexed
	codeFiles := []string{"main.go", "utils.ts", "helper.py", "config.yaml", "README.md", "data.json", "script.sh"}
	for _, file := range codeFiles {
		if _, ok := store.indexed[file]; !ok {
			t.Errorf("Code file %s should be indexed", file)
		}
	}

	// Verify non-code files were skipped
	if _, ok := store.indexed["ignored.txt"]; ok {
		t.Error(".txt files should be skipped")
	}

	if _, ok := store.indexed["binary.bin"]; ok {
		t.Error(".bin files should be skipped")
	}
}

func TestIndexRepository_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	dirs := []string{
		"internal/agent",
		"internal/gateway",
		"pkg/telemetry",
		"cmd/server",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}

		// Create a Go file in each dir
		file := filepath.Join(tmpDir, dir, "main.go")
		err = os.WriteFile(file, []byte("package main"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file in %s: %v", dir, err)
		}
	}

	store := newMockStore()
	indexer := NewIndexer(store, tmpDir, testLogger())

	err := indexer.IndexRepository(context.Background())
	if err != nil {
		t.Fatalf("IndexRepository failed: %v", err)
	}

	// Should index all nested files
	if store.indexCnt < 4 {
		t.Errorf("Expected at least 4 files indexed, got %d", store.indexCnt)
	}

	// Verify nested paths
	expectedPaths := []string{
		"internal/agent/main.go",
		"internal/gateway/main.go",
		"pkg/telemetry/main.go",
		"cmd/server/main.go",
	}

	for _, path := range expectedPaths {
		if _, ok := store.indexed[path]; !ok {
			t.Errorf("Nested file %s should be indexed", path)
		}
	}
}

