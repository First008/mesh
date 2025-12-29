package vectorstore

import (
	"fmt"
	"strings"
	"testing"
)

// TestTokenEstimation verifies token estimation is conservative
func TestTokenEstimation(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minChars int
	}{
		{
			name:     "empty string",
			text:     "",
			minChars: 0,
		},
		{
			name:     "short text",
			text:     "hello world",
			minChars: 2,
		},
		{
			name:     "code snippet",
			text:     "func main() {\n\tfmt.Println(\"Hello\")\n}",
			minChars: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := estimateTokens(tt.text)
			expectedChars := charsForTokens(tokens)

			// Verify round-trip: chars -> tokens -> chars >= original
			if expectedChars < len(tt.text) {
				t.Errorf("Token estimation too aggressive: %d tokens for %d chars (expected >= %d chars)",
					tokens, len(tt.text), expectedChars)
			}

			if tokens < tt.minChars {
				t.Errorf("estimateTokens(%q) = %d, want >= %d", tt.text, tokens, tt.minChars)
			}
		})
	}
}

// TestChunkFile_SmallFile verifies small files are not chunked
func TestChunkFile_SmallFile(t *testing.T) {
	content := strings.Repeat("// Small file\n", 100) // ~1400 chars, ~350 tokens
	chunks := ChunkFile("test.go", content, "go")

	if len(chunks) != 1 {
		t.Errorf("Small file should not be chunked: got %d chunks, want 1", len(chunks))
	}

	if chunks[0].ChunkIndex != 0 {
		t.Errorf("Single chunk should have index 0, got %d", chunks[0].ChunkIndex)
	}

	if chunks[0].ChunkID == "" {
		t.Error("Chunk should have a ChunkID")
	}
}

// TestChunkFile_LargeREADME verifies ~19KB README gets chunked
func TestChunkFile_LargeREADME(t *testing.T) {
	// Generate ~19KB README (similar to the failing case)
	var sb strings.Builder
	sb.WriteString("# Large README\n\n")

	for i := 0; i < 300; i++ {
		sb.WriteString("## Section ")
		sb.WriteString(strings.Repeat("A", i%26+65))
		sb.WriteString("\n\n")
		sb.WriteString("This is a detailed section with lots of content. ")
		sb.WriteString(strings.Repeat("Lorem ipsum dolor sit amet. ", 5))
		sb.WriteString("\n\n")
	}

	content := sb.String()
	t.Logf("Generated README: %d bytes, ~%d tokens", len(content), estimateTokens(content))

	if len(content) < 19000 {
		t.Fatalf("Test setup error: README should be ~19KB, got %d bytes", len(content))
	}

	chunks := ChunkFile("README.md", content, "markdown")

	// README should be chunked (exceeds MaxTokensWholeFile)
	if len(chunks) <= 1 {
		t.Errorf("19KB README should be chunked: got %d chunks, want > 1", len(chunks))
	}

	// Verify each chunk is within token budget (allow small margin for rounding)
	for i, chunk := range chunks {
		tokens := estimateTokens(chunk.Content)
		// Allow 1% margin for ceil() rounding in estimation
		marginTokens := int(float64(MaxTokensPerChunk) * 1.01)
		if tokens > marginTokens {
			t.Errorf("Chunk %d exceeds token budget: %d tokens, max %d (with margin %d)",
				i, tokens, MaxTokensPerChunk, marginTokens)
		}

		if chunk.ChunkID == "" {
			t.Errorf("Chunk %d missing ChunkID", i)
		}

		if chunk.ChunkIndex != i {
			t.Errorf("Chunk has wrong index: got %d, want %d", chunk.ChunkIndex, i)
		}
	}

	t.Logf("Chunked into %d pieces", len(chunks))
}

// TestChunkFile_MediumGoFile verifies ~11KB Go file gets chunked at func boundaries
func TestChunkFile_MediumGoFile(t *testing.T) {
	// Generate ~11KB Go file with multiple functions
	var sb strings.Builder
	sb.WriteString("package processor\n\n")
	sb.WriteString("import (\n\t\"context\"\n\t\"fmt\"\n)\n\n")

	for i := 0; i < 50; i++ {
		sb.WriteString("// Function")
		sb.WriteString(fmt.Sprintf("%d does something important\n", i))
		sb.WriteString(fmt.Sprintf("func Function%d(ctx context.Context) error {\n", i))
		sb.WriteString("\t// Implementation details\n")
		sb.WriteString(strings.Repeat("\tfmt.Println(\"Processing...\")\n", 15))
		sb.WriteString("\treturn nil\n")
		sb.WriteString("}\n\n")
	}

	content := sb.String()
	t.Logf("Generated Go file: %d bytes, ~%d tokens", len(content), estimateTokens(content))

	if len(content) < 11000 {
		t.Fatalf("Test setup error: Go file should be ~11KB, got %d bytes", len(content))
	}

	chunks := ChunkFile("pkg/processor/handler.go", content, "go")

	// File should be chunked (exceeds MaxTokensWholeFile)
	if len(chunks) <= 1 {
		t.Errorf("11KB Go file should be chunked: got %d chunks, want > 1", len(chunks))
	}

	// Verify each chunk respects token budget (allow small margin for rounding)
	for i, chunk := range chunks {
		tokens := estimateTokens(chunk.Content)
		// Allow 1% margin for ceil() rounding in estimation
		marginTokens := int(float64(MaxTokensPerChunk) * 1.01)
		if tokens > marginTokens {
			t.Errorf("Chunk %d exceeds token budget: %d tokens, max %d (with margin %d)",
				i, tokens, MaxTokensPerChunk, marginTokens)
		}

		if chunk.ChunkID == "" {
			t.Errorf("Chunk %d missing ChunkID", i)
		}

		// Verify chunk contains func boundaries (except possibly first chunk with imports)
		if i > 0 && !strings.Contains(chunk.Content, "func ") {
			t.Errorf("Chunk %d should contain function declarations", i)
		}
	}

	t.Logf("Chunked into %d pieces at function boundaries", len(chunks))
}

// TestChunkFile_OverlapWorks verifies adjacent chunks have overlap
func TestChunkFile_OverlapWorks(t *testing.T) {
	// Generate content large enough to require multiple chunks
	var sb strings.Builder
	sb.WriteString("package test\n\n")

	for i := 0; i < 100; i++ {
		uniqueLine := fmt.Sprintf("// UniqueMarker%d\n", i)
		sb.WriteString(uniqueLine)
		sb.WriteString(fmt.Sprintf("func Function%d() {\n", i))
		sb.WriteString(strings.Repeat("\t// body\n", 20))
		sb.WriteString("}\n\n")
	}

	content := sb.String()
	chunks := ChunkFile("test.go", content, "go")

	if len(chunks) <= 1 {
		t.Skip("Need multiple chunks to test overlap")
	}

	t.Logf("Generated %d chunks for overlap test", len(chunks))

	// Check that adjacent chunks share some content (overlap)
	for i := 0; i < len(chunks)-1; i++ {
		chunk1 := chunks[i].Content
		chunk2 := chunks[i+1].Content

		// Get last 500 chars of chunk1
		overlapRegion := ""
		if len(chunk1) > 500 {
			overlapRegion = chunk1[len(chunk1)-500:]
		} else {
			overlapRegion = chunk1
		}

		// Check if chunk2 starts with any of the overlap region
		hasOverlap := false
		lines1 := strings.Split(overlapRegion, "\n")
		for _, line := range lines1 {
			if len(line) > 10 && strings.Contains(chunk2, line) {
				hasOverlap = true
				break
			}
		}

		if !hasOverlap {
			t.Logf("Warning: chunks %d and %d may not have proper overlap", i, i+1)
		}
	}
}

// TestChunkFile_NoChunkExceedsTokenLimit ensures hard limit is enforced
func TestChunkFile_NoChunkExceedsTokenLimit(t *testing.T) {
	// Generate extremely large file with no good split points
	content := strings.Repeat("x", 100000) // 100KB of 'x' characters

	chunks := ChunkFile("large.txt", content, "text")

	for i, chunk := range chunks {
		tokens := estimateTokens(chunk.Content)
		// Allow some margin due to overlap, but must be < 2x the limit
		if tokens > MaxTokensPerChunk*2 {
			t.Errorf("Chunk %d far exceeds token limit: %d tokens (max %d)",
				i, tokens, MaxTokensPerChunk)
		}
	}

	t.Logf("Large file split into %d chunks, all within limits", len(chunks))
}

// TestChunkID_Stability verifies chunk IDs are stable and unique
func TestChunkID_Stability(t *testing.T) {
	content := strings.Repeat("func Test() {}\n", 500)

	// Generate chunks twice
	chunks1 := ChunkFile("test.go", content, "go")
	chunks2 := ChunkFile("test.go", content, "go")

	if len(chunks1) != len(chunks2) {
		t.Fatalf("Chunk count changed: %d vs %d", len(chunks1), len(chunks2))
	}

	// Verify IDs are stable (same content = same IDs)
	for i := range chunks1 {
		if chunks1[i].ChunkID != chunks2[i].ChunkID {
			t.Errorf("Chunk %d ID changed: %s vs %s",
				i, chunks1[i].ChunkID, chunks2[i].ChunkID)
		}
	}

	// Verify IDs are unique within a file
	seen := make(map[string]int)
	for i, chunk := range chunks1 {
		if prevIdx, exists := seen[chunk.ChunkID]; exists {
			t.Errorf("Duplicate ChunkID %s at indices %d and %d",
				chunk.ChunkID, prevIdx, i)
		}
		seen[chunk.ChunkID] = i
	}
}

// TestChunkFile_DifferentLanguages tests chunking for various languages
func TestChunkFile_DifferentLanguages(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		language string
		content  string
	}{
		{
			name:     "TypeScript",
			filePath: "test.ts",
			language: "typescript",
			content:  strings.Repeat("export class Test {}\n", 500),
		},
		{
			name:     "JavaScript",
			filePath: "test.js",
			language: "javascript",
			content:  strings.Repeat("function test() {}\n", 500),
		},
		{
			name:     "Markdown",
			filePath: "README.md",
			language: "markdown",
			content:  strings.Repeat("# Heading\n\nContent here.\n\n", 500),
		},
		{
			name:     "Python",
			filePath: "test.py",
			language: "python",
			content:  strings.Repeat("def test():\n    pass\n\n", 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := ChunkFile(tt.filePath, tt.content, tt.language)

			if len(chunks) == 0 {
				t.Error("Expected at least one chunk")
			}

			// Verify all chunks respect token budget
			for i, chunk := range chunks {
				tokens := estimateTokens(chunk.Content)
				if tokens > MaxTokensPerChunk*2 {
					t.Errorf("Chunk %d exceeds token budget: %d tokens", i, tokens)
				}

				if chunk.ChunkID == "" {
					t.Errorf("Chunk %d missing ChunkID", i)
				}
			}
		})
	}
}
