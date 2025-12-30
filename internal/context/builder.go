// Package context provides context building for LLM prompts.
//
// The context package implements layered context construction from repository
// code, supporting both cacheable (static) and regular (dynamic) context layers
// for prompt caching optimization.
package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/First008/mesh/internal/filetypes"
	"github.com/First008/mesh/internal/vectorstore"
	"github.com/rs/zerolog"
)

// Builder builds context for LLM queries from repository files
type Builder struct {
	repoPath        string
	repoName        string
	branch          string
	focusPaths      []string
	personality     string
	excludePatterns []string                // File patterns to exclude from results
	vectorStore     vectorstore.VectorStore // Optional: for semantic search (Phase 2+)
	logger          zerolog.Logger
}

// NewBuilder creates a new context builder (backward compatible)
func NewBuilder(repoPath, repoName string, focusPaths []string, logger zerolog.Logger) *Builder {
	return NewBuilderWithOptions(repoPath, repoName, "main", focusPaths, nil, nil, logger)
}

// NewBuilderWithBranch creates a new context builder with branch support (backward compatible)
func NewBuilderWithBranch(repoPath, repoName, branch string, focusPaths []string, vectorStore vectorstore.VectorStore, logger zerolog.Logger) *Builder {
	return NewBuilderWithOptions(repoPath, repoName, branch, focusPaths, nil, vectorStore, logger)
}

// NewBuilderWithOptions creates a new context builder with all options
func NewBuilderWithOptions(repoPath, repoName, branch string, focusPaths, excludePatterns []string, vectorStore vectorstore.VectorStore, logger zerolog.Logger) *Builder {
	return &Builder{
		repoPath:        repoPath,
		repoName:        repoName,
		branch:          branch,
		focusPaths:      focusPaths,
		excludePatterns: excludePatterns,
		vectorStore:     vectorStore,
		logger:          logger,
	}
}

// SetVectorStore sets the vector store for semantic search
func (b *Builder) SetVectorStore(store vectorstore.VectorStore) {
	b.vectorStore = store
	b.logger.Info().Msg("Vector store enabled for semantic search")
}

// SetExcludePatterns sets file exclusion patterns
func (b *Builder) SetExcludePatterns(patterns []string) {
	b.excludePatterns = patterns
	if len(patterns) > 0 {
		b.logger.Info().Strs("patterns", patterns).Msg("Exclude patterns configured")
	}
}

// SetPersonality sets the custom personality for this repository's agent
func (b *Builder) SetPersonality(personality string) {
	b.personality = personality
	b.logger.Debug().Str("personality", personality).Msg("Custom personality set")
}

// ContextLayers separates cacheable from regular context for prompt caching
type ContextLayers struct {
	Cacheable string // Static content (CLAUDE.md, README) - cached for 5min
	Regular   string // Dynamic content (code search results) - not cached
}

// BuildContextLayers builds context in layers for prompt caching optimization
func (b *Builder) BuildContextLayers(question string) (*ContextLayers, error) {
	var cacheableSB, regularSB strings.Builder

	// Layer 1 (Cacheable): CLAUDE.md - rarely changes
	claudeMD, err := b.loadClaudeMD()
	if err == nil && claudeMD != "" {
		cacheableSB.WriteString("# Project Context (from CLAUDE.md)\n\n")
		cacheableSB.WriteString(claudeMD)
		cacheableSB.WriteString("\n\n")
		b.logger.Debug().Msg("Loaded CLAUDE.md (cacheable)")
	}

	// Layer 1 (Cacheable): README.md - rarely changes
	readme, err := b.loadReadme()
	if err == nil && readme != "" {
		cacheableSB.WriteString("# README\n\n")
		cacheableSB.WriteString(readme)
		cacheableSB.WriteString("\n\n")
		b.logger.Debug().Msg("Loaded README.md (cacheable)")
	}

	// Layer 1 (Cacheable): Repository structure
	structure := b.getRepoStructure()
	if structure != "" {
		cacheableSB.WriteString("# Repository Structure\n\n")
		cacheableSB.WriteString(structure)
		cacheableSB.WriteString("\n\n")
	}

	// Layer 2 (Regular): Code search results - changes per query
	// Using 10 files for comprehensive context coverage
	relevantFiles, err := b.findRelevantFiles(question, 10)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to find relevant files")
	} else if len(relevantFiles) > 0 {
		// Log which files are being provided to the LLM
		fileNames := make([]string, len(relevantFiles))
		for i, f := range relevantFiles {
			fileNames[i] = f.RelPath
		}
		b.logger.Info().
			Strs("files", fileNames).
			Int("count", len(relevantFiles)).
			Msg("Context files for LLM")

		regularSB.WriteString("# Relevant Code Files\n\n")
		regularSB.WriteString("The following files are provided as context. ONLY reference these files in your answer:\n\n")
		for _, file := range relevantFiles {
			regularSB.WriteString(fmt.Sprintf("## %s\n\n", file.RelPath))
			regularSB.WriteString("```" + file.Language + "\n")
			regularSB.WriteString(file.Content)
			regularSB.WriteString("\n```\n\n")
		}
	}

	return &ContextLayers{
		Cacheable: cacheableSB.String(),
		Regular:   regularSB.String(),
	}, nil
}

// BuildContext builds context for a query by loading relevant files
// For backward compatibility - combines all context
func (b *Builder) BuildContext(question string) (string, error) {
	var sb strings.Builder

	// 1. Load CLAUDE.md if exists (project-specific instructions)
	claudeMD, err := b.loadClaudeMD()
	if err == nil && claudeMD != "" {
		sb.WriteString("# Project Context (from CLAUDE.md)\n\n")
		sb.WriteString(claudeMD)
		sb.WriteString("\n\n")
		b.logger.Debug().Msg("Loaded CLAUDE.md")
	}

	// 2. Load README.md (project overview)
	readme, err := b.loadReadme()
	if err == nil && readme != "" {
		sb.WriteString("# README\n\n")
		sb.WriteString(readme)
		sb.WriteString("\n\n")
		b.logger.Debug().Msg("Loaded README.md")
	}

	// 3. Load repository structure overview
	structure := b.getRepoStructure()
	if structure != "" {
		sb.WriteString("# Repository Structure\n\n")
		sb.WriteString(structure)
		sb.WriteString("\n\n")
	}

	// 4. Find relevant files using vector search (or keyword fallback)
	// Using 10 files for comprehensive context coverage
	relevantFiles, err := b.findRelevantFiles(question, 10)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to find relevant files")
	} else if len(relevantFiles) > 0 {
		sb.WriteString("# Relevant Code Files\n\n")
		for _, file := range relevantFiles {
			sb.WriteString(fmt.Sprintf("## %s\n\n", file.RelPath))
			sb.WriteString("```" + file.Language + "\n")
			sb.WriteString(file.Content)
			sb.WriteString("\n```\n\n")
		}
		b.logger.Debug().Int("file_count", len(relevantFiles)).Msg("Loaded relevant files")
	}

	context := sb.String()

	// Log context size for monitoring
	b.logger.Info().
		Int("context_bytes", len(context)).
		Int("context_chars", len(context)).
		Msg("Built context for query")

	return context, nil
}

// loadClaudeMD loads CLAUDE.md from common locations
func (b *Builder) loadClaudeMD() (string, error) {
	paths := []string{
		filepath.Join(b.repoPath, "CLAUDE.md"),
		filepath.Join(b.repoPath, ".claude", "CLAUDE.md"),
		filepath.Join(b.repoPath, "docs", "CLAUDE.md"),
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("CLAUDE.md not found")
}

// loadReadme loads README.md from repository root
func (b *Builder) loadReadme() (string, error) {
	paths := []string{
		filepath.Join(b.repoPath, "README.md"),
		filepath.Join(b.repoPath, "readme.md"),
		filepath.Join(b.repoPath, "README"),
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("README not found")
}

// getRepoStructure returns a simple directory tree of the repository
func (b *Builder) getRepoStructure() string {
	var sb strings.Builder

	sb.WriteString("```\n")
	sb.WriteString(b.repoName + "/\n")

	// Walk focus paths to show structure
	seen := make(map[string]bool)
	for _, focusPath := range b.focusPaths {
		pattern := filepath.Join(b.repoPath, focusPath)

		// Simple directory listing (not full tree to keep context small)
		dirs := b.listDirectories(pattern)
		for _, dir := range dirs {
			if !seen[dir] {
				sb.WriteString("├── " + dir + "/\n")
				seen[dir] = true
			}
		}
	}

	sb.WriteString("```\n")

	return sb.String()
}

// listDirectories returns top-level directories matching pattern
func (b *Builder) listDirectories(pattern string) []string {
	var dirs []string

	// Extract base path without wildcards
	basePath := b.repoPath

	// List directories in base path
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return dirs
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs
}

// FileInfo holds information about a relevant file
type FileInfo struct {
	RelPath  string
	Content  string
	Language string
}

// findRelevantFiles finds files relevant to the question
// Phase 2: Uses vector search if available, falls back to keyword matching
func (b *Builder) findRelevantFiles(question string, limit int) ([]FileInfo, error) {
	// If vector store is available, use semantic search
	if b.vectorStore != nil {
		return b.vectorSearch(question, limit)
	}

	// Fallback to keyword search (Phase 1)
	return b.keywordSearch(question, limit)
}

// vectorSearch uses the vector store for semantic search
// Uses aggregated search to reconstruct complete files from chunks
func (b *Builder) vectorSearch(question string, limit int) ([]FileInfo, error) {
	var results []vectorstore.SearchResult
	var err error

	// Use aggregated search (reconstructs complete files from chunks)
	// All VectorStore implementations now support this via the interface
	results, err = b.vectorStore.SearchWithAggregation(context.Background(), question, limit)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Aggregated search failed, falling back to basic search")
		results, err = b.vectorStore.Search(context.Background(), question, limit)
	}

	if err != nil {
		b.logger.Warn().Err(err).Msg("Vector search failed, falling back to keyword search")
		return b.keywordSearch(question, limit)
	}

	var files []FileInfo
	for _, result := range results {
		// Apply exclude patterns
		if b.shouldExclude(result.FilePath) {
			b.logger.Debug().
				Str("file", result.FilePath).
				Msg("Excluded file based on exclude patterns")
			continue
		}

		files = append(files, FileInfo{
			RelPath:  result.FilePath,
			Content:  result.Content,
			Language: result.Language,
		})
	}

	b.logger.Debug().
		Int("result_count", len(files)).
		Str("search_method", "vector_aggregated").
		Msg("Found relevant files")

	return files, nil
}

// shouldExclude checks if a file should be excluded based on exclude patterns
func (b *Builder) shouldExclude(filePath string) bool {
	if len(b.excludePatterns) == 0 {
		return false
	}

	for _, pattern := range b.excludePatterns {
		// Support glob patterns using filepath.Match
		if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
			return true
		}
		// Also support substring matching for paths
		if strings.Contains(filePath, pattern) {
			return true
		}
	}
	return false
}

// keywordSearch performs simple keyword-based search
func (b *Builder) keywordSearch(question string, limit int) ([]FileInfo, error) {
	keywords := b.extractKeywords(question)

	var files []FileInfo
	var searchPaths []string

	// Determine search paths from focus paths
	if len(b.focusPaths) > 0 {
		for _, focusPath := range b.focusPaths {
			// Remove wildcards for simple search
			cleanPath := strings.ReplaceAll(focusPath, "**", "")
			cleanPath = strings.ReplaceAll(cleanPath, "*", "")
			searchPath := filepath.Join(b.repoPath, cleanPath)
			searchPaths = append(searchPaths, searchPath)
		}
	} else {
		// No focus paths specified, search entire repo
		searchPaths = []string{b.repoPath}
	}

	// Walk each search path
	for _, searchPath := range searchPaths {
		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if info.IsDir() {
				// Skip common non-code directories
				if filetypes.ShouldSkipDirectory(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			// Only process code files
			if !b.isCodeFile(path) {
				return nil
			}

			// Check if file content matches keywords
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			contentStr := string(content)
			score := b.scoreContent(contentStr, keywords)

			if score > 0 {
				relPath, _ := filepath.Rel(b.repoPath, path)
				files = append(files, FileInfo{
					RelPath:  relPath,
					Content:  contentStr,
					Language: b.getLanguage(path),
				})
			}

			return nil
		})
		if err != nil {
			b.logger.Warn().Err(err).Str("path", searchPath).Msg("Error walking path")
		}
	}

	// Sort by score and return top N
	// For Phase 1, we'll just return first N files found
	if len(files) > limit {
		files = files[:limit]
	}

	b.logger.Debug().
		Int("result_count", len(files)).
		Str("search_method", "keyword").
		Msg("Found relevant files")

	return files, nil
}

// extractKeywords extracts meaningful keywords from a question
func (b *Builder) extractKeywords(question string) []string {
	// Convert to lowercase
	question = strings.ToLower(question)

	// Split into words
	words := strings.FieldsFunc(question, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_')
	})

	// Filter out stop words
	stopWords := map[string]bool{
		"what": true, "how": true, "where": true, "when": true, "why": true,
		"does": true, "is": true, "are": true, "the": true, "a": true, "an": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true,
		"this": true, "that": true, "these": true, "those": true,
		"do": true, "did": true, "can": true, "could": true, "would": true, "should": true,
	}

	var keywords []string
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// scoreContent scores content based on keyword matches
func (b *Builder) scoreContent(content string, keywords []string) int {
	contentLower := strings.ToLower(content)
	score := 0

	for _, keyword := range keywords {
		if strings.Contains(contentLower, keyword) {
			score++
		}
	}

	return score
}

// isCodeFile checks if a file is a code file we should index
// Delegates to filetypes package (single source of truth)
func (b *Builder) isCodeFile(path string) bool {
	return filetypes.IsCodeFile(path)
}

// getLanguage returns the language identifier for syntax highlighting
// Delegates to filetypes package (single source of truth)
func (b *Builder) getLanguage(path string) string {
	lang := filetypes.GetLanguage(path)
	if lang == "" {
		return "text"
	}
	return lang
}
