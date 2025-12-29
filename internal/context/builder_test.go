package context

import (
	"context"
	"io"
	"testing"

	"github.com/First008/mesh/internal/vectorstore"
	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

// Local mock to avoid import cycles
type mockVectorStore struct {
	indexedFiles    map[string]string
	SearchCallCount int
	LastQuery       string
	SearchFunc      func(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error)
}

func newMockVectorStore() *mockVectorStore {
	return &mockVectorStore{
		indexedFiles: make(map[string]string),
	}
}

func (m *mockVectorStore) IndexFile(ctx context.Context, filePath, content string) error {
	m.indexedFiles[filePath] = content
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error) {
	m.SearchCallCount++
	m.LastQuery = query

	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query, limit)
	}

	results := []vectorstore.SearchResult{}
	for path, content := range m.indexedFiles {
		results = append(results, vectorstore.SearchResult{
			FilePath: path,
			Content:  content,
			Score:    0.85,
			Language: "go",
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (m *mockVectorStore) DeleteFile(ctx context.Context, filePath string) error {
	delete(m.indexedFiles, filePath)
	return nil
}

func (m *mockVectorStore) DeleteCollection(ctx context.Context) error {
	m.indexedFiles = make(map[string]string)
	return nil
}

func (m *mockVectorStore) GetStats(ctx context.Context) (*vectorstore.Stats, error) {
	return &vectorstore.Stats{
		TotalVectors: int64(len(m.indexedFiles)),
	}, nil
}

func (m *mockVectorStore) SearchWithAggregation(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error) {
	// Delegate to Search for test mocks
	return m.Search(ctx, query, limit)
}

func (m *mockVectorStore) Close() error {
	return nil
}

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder("/tmp/test-repo", "test-repo", []string{"internal/", "pkg/"}, testLogger())

	if builder == nil {
		t.Fatal("Expected builder, got nil")
	}

	if builder.repoPath != "/tmp/test-repo" {
		t.Errorf("Expected repo path '/tmp/test-repo', got '%s'", builder.repoPath)
	}

	if builder.repoName != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got '%s'", builder.repoName)
	}

	if len(builder.focusPaths) != 2 {
		t.Errorf("Expected 2 focus paths, got %d", len(builder.focusPaths))
	}

	// Default branch should be "main"
	if builder.branch != "main" {
		t.Errorf("Expected default branch 'main', got '%s'", builder.branch)
	}
}

func TestNewBuilderWithBranch(t *testing.T) {
	builder := NewBuilderWithBranch(
		"/tmp/test-repo",
		"test-repo",
		"develop",
		[]string{"src/"},
		nil,
		testLogger(),
	)

	if builder == nil {
		t.Fatal("Expected builder, got nil")
	}

	if builder.branch != "develop" {
		t.Errorf("Expected branch 'develop', got '%s'", builder.branch)
	}

	if builder.repoPath != "/tmp/test-repo" {
		t.Errorf("Expected repo path '/tmp/test-repo', got '%s'", builder.repoPath)
	}

	if builder.repoName != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got '%s'", builder.repoName)
	}
}

func TestNewBuilderWithBranch_WithVectorStore(t *testing.T) {
	mockStore := newMockVectorStore()

	builder := NewBuilderWithBranch(
		"/tmp/test-repo",
		"test-repo",
		"main",
		nil,
		mockStore,
		testLogger(),
	)

	if builder == nil {
		t.Fatal("Expected builder, got nil")
	}

	if builder.vectorStore == nil {
		t.Error("Expected vector store to be set")
	}
}

func TestSetVectorStore(t *testing.T) {
	builder := NewBuilder("/tmp/test-repo", "test-repo", nil, testLogger())

	if builder.vectorStore != nil {
		t.Error("Expected nil vector store initially")
	}

	mockStore := newMockVectorStore()
	builder.SetVectorStore(mockStore)

	if builder.vectorStore == nil {
		t.Error("Expected vector store to be set")
	}

	// Verify we can use the store
	_, err := builder.vectorStore.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Errorf("Vector store Search failed: %v", err)
	}
}

func TestSetPersonality(t *testing.T) {
	builder := NewBuilder("/tmp/test-repo", "test-repo", nil, testLogger())

	if builder.personality != "" {
		t.Error("Expected empty personality initially")
	}

	customPersonality := "You are a helpful Go expert."
	builder.SetPersonality(customPersonality)

	if builder.personality != customPersonality {
		t.Errorf("Expected personality '%s', got '%s'", customPersonality, builder.personality)
	}
}

func TestNewBuilder_EmptyFocusPaths(t *testing.T) {
	builder := NewBuilder("/tmp/test-repo", "test-repo", nil, testLogger())

	if builder == nil {
		t.Fatal("Expected builder, got nil")
	}

	if builder.focusPaths != nil && len(builder.focusPaths) > 0 {
		t.Error("Expected nil or empty focus paths")
	}
}

func TestNewBuilder_MultipleFocusPaths(t *testing.T) {
	focusPaths := []string{"internal/", "pkg/", "cmd/", "api/"}
	builder := NewBuilder("/tmp/test-repo", "test-repo", focusPaths, testLogger())

	if builder == nil {
		t.Fatal("Expected builder, got nil")
	}

	if len(builder.focusPaths) != 4 {
		t.Errorf("Expected 4 focus paths, got %d", len(builder.focusPaths))
	}

	for i, path := range focusPaths {
		if builder.focusPaths[i] != path {
			t.Errorf("Focus path %d: expected '%s', got '%s'", i, path, builder.focusPaths[i])
		}
	}
}

func TestBuildContextLayers_WithMockVectorStore(t *testing.T) {
	// Create mock vector store with some test data
	mockStore := newMockVectorStore()

	// Add some indexed files
	_ = mockStore.IndexFile(context.Background(), "main.go", "package main\nfunc main() {}")
	_ = mockStore.IndexFile(context.Background(), "utils.go", "package utils\nfunc Helper() {}")

	builder := NewBuilderWithBranch(
		"/tmp/test-repo",
		"test-repo",
		"main",
		nil,
		mockStore,
		testLogger(),
	)

	// Build context layers
	layers, err := builder.BuildContextLayers("What does main.go do?")
	if err != nil {
		t.Fatalf("BuildContextLayers failed: %v", err)
	}

	if layers == nil {
		t.Fatal("Expected context layers, got nil")
	}

	// Should have at least searched the vector store
	if mockStore.SearchCallCount == 0 {
		t.Error("Expected vector store to be searched")
	}
}

func TestBuildContextLayers_WithoutVectorStore(t *testing.T) {
	builder := NewBuilder("/tmp/test-repo", "test-repo", nil, testLogger())

	// Build context layers without vector store (will use keyword search)
	layers, err := builder.BuildContextLayers("What is this repo about?")
	if err != nil {
		t.Fatalf("BuildContextLayers failed: %v", err)
	}

	if layers == nil {
		t.Fatal("Expected context layers, got nil")
	}

	// Layers should be non-nil but may be empty (no files in /tmp/test-repo)
	// This is expected behavior when repo doesn't exist
}

func TestBuilderFields(t *testing.T) {
	repoPath := "/home/user/projects/myrepo"
	repoName := "myrepo"
	branch := "feature-branch"
	focusPaths := []string{"src/", "lib/"}

	builder := NewBuilderWithBranch(repoPath, repoName, branch, focusPaths, nil, testLogger())

	// Verify all fields are set correctly
	if builder.repoPath != repoPath {
		t.Errorf("repoPath: expected '%s', got '%s'", repoPath, builder.repoPath)
	}

	if builder.repoName != repoName {
		t.Errorf("repoName: expected '%s', got '%s'", repoName, builder.repoName)
	}

	if builder.branch != branch {
		t.Errorf("branch: expected '%s', got '%s'", branch, builder.branch)
	}

	if len(builder.focusPaths) != len(focusPaths) {
		t.Errorf("focusPaths length: expected %d, got %d", len(focusPaths), len(builder.focusPaths))
	}
}

func TestContextLayers_Structure(t *testing.T) {
	// Test that ContextLayers struct works as expected
	layers := &ContextLayers{
		Cacheable: "# Static Content\nREADME content here",
		Regular:   "# Dynamic Content\nSearch results here",
	}

	if layers.Cacheable == "" {
		t.Error("Cacheable should not be empty")
	}

	if layers.Regular == "" {
		t.Error("Regular should not be empty")
	}
}

func TestSetVectorStore_ReplacesExisting(t *testing.T) {
	builder := NewBuilder("/tmp/test-repo", "test-repo", nil, testLogger())

	// Set first vector store
	mockStore1 := newMockVectorStore()
	_ = mockStore1.IndexFile(context.Background(), "file1.go", "content1")
	builder.SetVectorStore(mockStore1)

	// Verify first store is set
	stats1, _ := builder.vectorStore.GetStats(context.Background())
	if stats1.TotalVectors != 1 {
		t.Errorf("Expected 1 vector in first store, got %d", stats1.TotalVectors)
	}

	// Replace with second vector store
	mockStore2 := newMockVectorStore()
	_ = mockStore2.IndexFile(context.Background(), "file1.go", "content1")
	_ = mockStore2.IndexFile(context.Background(), "file2.go", "content2")
	builder.SetVectorStore(mockStore2)

	// Verify second store replaced first
	stats2, _ := builder.vectorStore.GetStats(context.Background())
	if stats2.TotalVectors != 2 {
		t.Errorf("Expected 2 vectors in second store, got %d", stats2.TotalVectors)
	}
}

func TestBuildContextLayers_VectorStoreSearch(t *testing.T) {
	mockStore := newMockVectorStore()

	// Define search behavior
	mockStore.SearchFunc = func(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error) {
		return []vectorstore.SearchResult{
			{
				FilePath: "src/main.go",
				Content:  "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
				Score:    0.95,
				Language: "go",
			},
		}, nil
	}

	builder := NewBuilderWithBranch("/tmp/test-repo", "test-repo", "main", nil, mockStore, testLogger())

	layers, err := builder.BuildContextLayers("show me the main function")
	if err != nil {
		t.Fatalf("BuildContextLayers failed: %v", err)
	}

	// Verify search was called
	if mockStore.SearchCallCount == 0 {
		t.Error("Expected vector store search to be called")
	}

	// Verify last query contains our question keywords
	if mockStore.LastQuery == "" {
		t.Error("Expected search query to be recorded")
	}

	// Layers should be populated
	if layers == nil {
		t.Fatal("Expected non-nil layers")
	}
}

// Note: Full testing of BuildContextLayers requires filesystem access
// or more sophisticated mocking. The tests above cover the core
// initialization and integration points. After Phase 4 (removing duplication),
// we can add more comprehensive tests with test fixtures.
