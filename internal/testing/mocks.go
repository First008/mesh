// Package testing provides test utilities, mocks, and fixtures for testing MESH components.
package testing

import (
	"context"
	"fmt"

	"github.com/First008/mesh/internal/llm"
	"github.com/First008/mesh/internal/vectorstore"
)

// MockLLMProvider is a mock implementation of llm.LLMProvider for testing
// without making real API calls.
type MockLLMProvider struct {
	// AskFunc is called when Ask() is invoked. If nil, returns default response.
	AskFunc func(ctx context.Context, system, user string) (*llm.Response, error)

	// AskWithCacheFunc is called when AskWithCache() is invoked. If nil, returns default response.
	AskWithCacheFunc func(ctx context.Context, system, cacheable, regular, question string) (*llm.Response, error)

	// CountTokensFunc is called when CountTokens() is invoked. If nil, returns length/4 as estimate.
	CountTokensFunc func(text string) (int, error)

	// Model is the model name to return from GetModel()
	Model string

	// SupportsCaching controls the return value of SupportsPromptCaching()
	SupportsCaching bool

	// CallCount tracks how many times Ask/AskWithCache was called
	CallCount int

	// LastSystemPrompt stores the last system prompt received
	LastSystemPrompt string

	// LastUserPrompt stores the last user prompt received
	LastUserPrompt string
}

// Ask implements llm.LLMProvider.Ask
func (m *MockLLMProvider) Ask(ctx context.Context, systemPrompt, userPrompt string) (*llm.Response, error) {
	m.CallCount++
	m.LastSystemPrompt = systemPrompt
	m.LastUserPrompt = userPrompt

	if m.AskFunc != nil {
		return m.AskFunc(ctx, systemPrompt, userPrompt)
	}

	// Default response
	return &llm.Response{
		Content:      "Mock response from " + m.GetModel(),
		InputTokens:  100,
		OutputTokens: 50,
		CachedTokens: 0,
		Model:        m.GetModel(),
	}, nil
}

// AskWithCache implements llm.LLMProvider.AskWithCache
func (m *MockLLMProvider) AskWithCache(ctx context.Context, systemPrompt, cacheableContext, regularContext, question string) (*llm.Response, error) {
	m.CallCount++
	m.LastSystemPrompt = systemPrompt
	m.LastUserPrompt = question

	if m.AskWithCacheFunc != nil {
		return m.AskWithCacheFunc(ctx, systemPrompt, cacheableContext, regularContext, question)
	}

	// Default response with cache simulation
	return &llm.Response{
		Content:      "Mock cached response from " + m.GetModel(),
		InputTokens:  50, // Less input tokens due to caching
		OutputTokens: 50,
		CachedTokens: 500, // Simulated cached tokens
		Model:        m.GetModel(),
	}, nil
}

// CountTokens implements llm.LLMProvider.CountTokens
func (m *MockLLMProvider) CountTokens(text string) (int, error) {
	if m.CountTokensFunc != nil {
		return m.CountTokensFunc(text)
	}

	// Simple estimation: ~4 chars per token
	return len(text) / 4, nil
}

// GetModel implements llm.LLMProvider.GetModel
func (m *MockLLMProvider) GetModel() string {
	if m.Model == "" {
		return "mock-model-v1"
	}
	return m.Model
}

// SupportsPromptCaching implements llm.LLMProvider.SupportsPromptCaching
func (m *MockLLMProvider) SupportsPromptCaching() bool {
	return m.SupportsCaching
}

// MockVectorStore is a mock implementation of vectorstore.VectorStore for testing
// without a real vector database.
type MockVectorStore struct {
	// IndexFileFunc is called when IndexFile() is invoked
	IndexFileFunc func(ctx context.Context, filePath, content string) error

	// SearchFunc is called when Search() is invoked
	SearchFunc func(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error)

	// DeleteFileFunc is called when DeleteFile() is invoked
	DeleteFileFunc func(ctx context.Context, filePath string) error

	// DeleteCollectionFunc is called when DeleteCollection() is invoked
	DeleteCollectionFunc func(ctx context.Context) error

	// GetStatsFunc is called when GetStats() is invoked
	GetStatsFunc func(ctx context.Context) (*vectorstore.Stats, error)

	// CloseFunc is called when Close() is invoked
	CloseFunc func() error

	// IndexedFiles tracks files that have been indexed
	IndexedFiles map[string]string

	// CallCount tracks total method calls
	CallCount int

	// SearchCallCount tracks search calls separately
	SearchCallCount int

	// LastQuery stores the last search query
	LastQuery string
}

// NewMockVectorStore creates a new MockVectorStore with initialized fields
func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{
		IndexedFiles: make(map[string]string),
	}
}

// IndexFile implements vectorstore.VectorStore.IndexFile
func (m *MockVectorStore) IndexFile(ctx context.Context, filePath, content string) error {
	m.CallCount++

	if m.IndexFileFunc != nil {
		return m.IndexFileFunc(ctx, filePath, content)
	}

	// Default: store in memory
	m.IndexedFiles[filePath] = content
	return nil
}

// Search implements vectorstore.VectorStore.Search
func (m *MockVectorStore) Search(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error) {
	m.CallCount++
	m.SearchCallCount++
	m.LastQuery = query

	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query, limit)
	}

	// Default: return first N indexed files
	results := []vectorstore.SearchResult{}
	count := 0
	for path, content := range m.IndexedFiles {
		if count >= limit {
			break
		}
		results = append(results, vectorstore.SearchResult{
			FilePath: path,
			Content:  content,
			Score:    0.85,
			Language: "go",
			FileHash: "mock-hash-" + path,
		})
		count++
	}

	return results, nil
}

// SearchWithAggregation implements vectorstore.VectorStore.SearchWithAggregation
// For mocks, this just delegates to Search (no chunking in tests)
func (m *MockVectorStore) SearchWithAggregation(ctx context.Context, query string, limit int) ([]vectorstore.SearchResult, error) {
	return m.Search(ctx, query, limit)
}

// DeleteFile implements vectorstore.VectorStore.DeleteFile
func (m *MockVectorStore) DeleteFile(ctx context.Context, filePath string) error {
	m.CallCount++

	if m.DeleteFileFunc != nil {
		return m.DeleteFileFunc(ctx, filePath)
	}

	delete(m.IndexedFiles, filePath)
	return nil
}

// DeleteCollection implements vectorstore.VectorStore.DeleteCollection
func (m *MockVectorStore) DeleteCollection(ctx context.Context) error {
	m.CallCount++

	if m.DeleteCollectionFunc != nil {
		return m.DeleteCollectionFunc(ctx)
	}

	m.IndexedFiles = make(map[string]string)
	return nil
}

// GetStats implements vectorstore.VectorStore.GetStats
func (m *MockVectorStore) GetStats(ctx context.Context) (*vectorstore.Stats, error) {
	m.CallCount++

	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx)
	}

	return &vectorstore.Stats{
		TotalVectors: int64(len(m.IndexedFiles)),
	}, nil
}

// Close implements vectorstore.VectorStore.Close
func (m *MockVectorStore) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockEmbeddingProvider is a mock implementation of vectorstore.EmbeddingProvider
// for testing indexing without real embedding models.
type MockEmbeddingProvider struct {
	// CreateEmbeddingFunc is called when CreateEmbedding() is invoked
	CreateEmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

	// Dimensions is the embedding vector dimensionality
	Dimensions int

	// ModelName is the model name to return
	ModelName string

	// CallCount tracks how many embeddings were created
	CallCount int

	// LastText stores the last text embedded
	LastText string
}

// NewMockEmbeddingProvider creates a new MockEmbeddingProvider with sensible defaults
func NewMockEmbeddingProvider() *MockEmbeddingProvider {
	return &MockEmbeddingProvider{
		Dimensions: 768, // Common embedding size
		ModelName:  "mock-embedding-v1",
	}
}

// CreateEmbedding implements vectorstore.EmbeddingProvider.CreateEmbedding
func (m *MockEmbeddingProvider) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.CallCount++
	m.LastText = text

	if m.CreateEmbeddingFunc != nil {
		return m.CreateEmbeddingFunc(ctx, text)
	}

	// Default: return a fake embedding vector
	// Use deterministic values based on text length for consistency
	embedding := make([]float32, m.GetDimensions())
	baseValue := float32(len(text)%100) / 100.0
	for i := range embedding {
		embedding[i] = baseValue + float32(i)/float32(len(embedding))
	}

	return embedding, nil
}

// GetDimensions implements vectorstore.EmbeddingProvider.GetDimensions
func (m *MockEmbeddingProvider) GetDimensions() int {
	if m.Dimensions == 0 {
		return 768
	}
	return m.Dimensions
}

// GetModelName implements vectorstore.EmbeddingProvider.GetModelName
func (m *MockEmbeddingProvider) GetModelName() string {
	if m.ModelName == "" {
		return "mock-embedding-v1"
	}
	return m.ModelName
}

// ErrorLLMProvider is a mock that always returns errors (for error testing)
type ErrorLLMProvider struct {
	ErrorMessage string
}

// Ask always returns an error
func (e *ErrorLLMProvider) Ask(ctx context.Context, system, user string) (*llm.Response, error) {
	return nil, fmt.Errorf("%s", e.ErrorMessage)
}

// AskWithCache always returns an error
func (e *ErrorLLMProvider) AskWithCache(ctx context.Context, system, cacheable, regular, question string) (*llm.Response, error) {
	return nil, fmt.Errorf("%s", e.ErrorMessage)
}

// CountTokens always returns an error
func (e *ErrorLLMProvider) CountTokens(text string) (int, error) {
	return 0, fmt.Errorf("%s", e.ErrorMessage)
}

// GetModel returns error model name
func (e *ErrorLLMProvider) GetModel() string {
	return "error-model"
}

// SupportsPromptCaching returns false
func (e *ErrorLLMProvider) SupportsPromptCaching() bool {
	return false
}
