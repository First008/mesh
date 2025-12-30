package vectorstore

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
)

// Token budget configuration for bge-m3 model (8192 token context)
const (
	MaxTokensPerChunk     = 3500 // Safe chunk size to avoid context limit
	OverlapTokens         = 250  // Overlap between chunks for context continuity
	MaxTokensWholeFile    = 3200 // Embed whole file if under this limit
	CharsPerTokenEstimate = 4.0  // Conservative estimate: 1 token ≈ 4 chars
)

// CodeChunk represents a chunk of code with metadata
type CodeChunk struct {
	Content    string
	ChunkIndex int
	StartLine  int
	EndLine    int
	Header     string // Context header for better retrieval
	ChunkID    string // Stable identifier: hash(path + startLine + endLine)
}

// estimateTokens provides conservative token count estimate
// Uses ~4 chars per token as a safe approximation for code/text
func estimateTokens(text string) int {
	return int(math.Ceil(float64(len(text)) / CharsPerTokenEstimate))
}

// charsForTokens converts token count to approximate character count
func charsForTokens(tokens int) int {
	return int(float64(tokens) * CharsPerTokenEstimate)
}

// ChunkFile splits a file into semantically meaningful chunks
// Uses token-budget-based chunking to ensure no chunk exceeds model limits
func ChunkFile(filePath, content, language string) []CodeChunk {
	// Convert token budgets to character counts (conservative approximation)
	maxChunkChars := charsForTokens(MaxTokensPerChunk)
	overlapChars := charsForTokens(OverlapTokens)
	minChunkChars := 500 // Skip tiny chunks

	// Check if file fits within safe token budget for whole-file embedding
	fileTokens := estimateTokens(content)
	if fileTokens <= MaxTokensWholeFile {
		chunk := CodeChunk{
			Content:    content,
			ChunkIndex: 0,
			StartLine:  1,
			EndLine:    countLines(content),
			Header:     buildHeader(filePath, language, ""),
		}
		chunk.ChunkID = generateChunkID(filePath, chunk.StartLine, chunk.EndLine)
		return []CodeChunk{chunk}
	}

	// File exceeds token budget - chunk it
	var chunks []CodeChunk

	switch language {
	case "go":
		chunks = chunkGoCode(filePath, content, maxChunkChars, overlapChars)
	case "typescript", "javascript":
		chunks = chunkTSCode(filePath, content, maxChunkChars, overlapChars)
	default:
		// Fallback: simple line-based chunking
		chunks = chunkByLines(filePath, content, language, maxChunkChars, overlapChars)
	}

	// Filter out chunks that are too small and generate chunk IDs
	filtered := make([]CodeChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk.Content) >= minChunkChars {
			chunk.ChunkID = generateChunkID(filePath, chunk.StartLine, chunk.EndLine)
			filtered = append(filtered, chunk)
		}
	}

	return filtered
}

// chunkGoCode splits Go code by top-level declarations
func chunkGoCode(filePath, content string, maxSize, overlap int) []CodeChunk {
	lines := strings.Split(content, "\n")
	var chunks []CodeChunk
	var currentChunk strings.Builder
	currentLine := 1
	chunkStartLine := 1
	chunkIndex := 0

	for i, line := range lines {
		// Detect top-level declarations
		trimmed := strings.TrimSpace(line)
		isTopLevel := strings.HasPrefix(trimmed, "func ") ||
			strings.HasPrefix(trimmed, "type ") ||
			strings.HasPrefix(trimmed, "const ") ||
			strings.HasPrefix(trimmed, "var ") ||
			strings.HasPrefix(trimmed, "package ") ||
			strings.HasPrefix(trimmed, "import ")

		// Check if adding this line would exceed maxSize
		willExceed := currentChunk.Len()+len(line)+1 > maxSize

		// Hard limit: force split if chunk reaches maxSize (strict enforcement)
		hardLimit := currentChunk.Len() >= maxSize

		// Prefer split at: (1) good boundary when approaching limit, or (2) hard limit
		shouldSplit := (willExceed && isTopLevel) || hardLimit

		if shouldSplit && currentChunk.Len() > 0 {
			// Save current chunk
			chunkContent := currentChunk.String()
			chunks = append(chunks, CodeChunk{
				Content:    chunkContent,
				ChunkIndex: chunkIndex,
				StartLine:  chunkStartLine,
				EndLine:    currentLine - 1,
				Header:     buildHeader(filePath, "go", extractSymbol(chunkContent)),
			})

			// Start new chunk with overlap
			currentChunk.Reset()
			overlapLines := getOverlapLines(lines, i, overlap)
			currentChunk.WriteString(overlapLines)
			chunkStartLine = i - strings.Count(overlapLines, "\n")
			if chunkStartLine < 1 {
				chunkStartLine = 1
			}
			chunkIndex++
		}

		// Add current line to chunk
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
		currentLine++
	}

	// Add final chunk
	if currentChunk.Len() > 0 {
		chunkContent := currentChunk.String()
		chunks = append(chunks, CodeChunk{
			Content:    chunkContent,
			ChunkIndex: chunkIndex,
			StartLine:  chunkStartLine,
			EndLine:    currentLine - 1,
			Header:     buildHeader(filePath, "go", extractSymbol(chunkContent)),
		})
	}

	return chunks
}

// chunkTSCode splits TypeScript/JavaScript code by exports and classes
func chunkTSCode(filePath, content string, maxSize, overlap int) []CodeChunk {
	lines := strings.Split(content, "\n")
	var chunks []CodeChunk
	var currentChunk strings.Builder
	currentLine := 1
	chunkStartLine := 1
	chunkIndex := 0

	for i, line := range lines {
		// Check if adding this line would exceed maxSize
		willExceed := currentChunk.Len()+len(line)+1 > maxSize

		trimmed := strings.TrimSpace(line)
		isTopLevel := strings.HasPrefix(trimmed, "export ") ||
			strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "function ") ||
			strings.HasPrefix(trimmed, "const ") ||
			strings.HasPrefix(trimmed, "interface ") ||
			strings.HasPrefix(trimmed, "type ")

		// Hard limit: force split if chunk reaches maxSize (strict enforcement)
		hardLimit := currentChunk.Len() >= maxSize

		// Prefer split at: (1) good boundary when approaching limit, or (2) hard limit
		shouldSplit := (willExceed && isTopLevel) || hardLimit

		if shouldSplit && currentChunk.Len() > 0 {
			chunkContent := currentChunk.String()
			chunks = append(chunks, CodeChunk{
				Content:    chunkContent,
				ChunkIndex: chunkIndex,
				StartLine:  chunkStartLine,
				EndLine:    currentLine - 1,
				Header:     buildHeader(filePath, "typescript", extractSymbol(chunkContent)),
			})

			currentChunk.Reset()
			overlapLines := getOverlapLines(lines, i, overlap)
			currentChunk.WriteString(overlapLines)
			chunkStartLine = i - strings.Count(overlapLines, "\n")
			if chunkStartLine < 1 {
				chunkStartLine = 1
			}
			chunkIndex++
		}

		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
		currentLine++
	}

	if currentChunk.Len() > 0 {
		chunkContent := currentChunk.String()
		chunks = append(chunks, CodeChunk{
			Content:    chunkContent,
			ChunkIndex: chunkIndex,
			StartLine:  chunkStartLine,
			EndLine:    currentLine - 1,
			Header:     buildHeader(filePath, "typescript", extractSymbol(chunkContent)),
		})
	}

	return chunks
}

// chunkByLines simple line-based chunking for unsupported languages
func chunkByLines(filePath, content, language string, maxSize, overlap int) []CodeChunk {
	lines := strings.Split(content, "\n")
	var chunks []CodeChunk
	chunkIndex := 0

	for i := 0; i < len(lines); {
		var chunk strings.Builder
		startLine := i + 1

		// Take lines until maxSize, with hard limit enforcement
		for i < len(lines) {
			line := lines[i]

			// If single line exceeds maxSize, split it by character boundaries
			if len(line) > maxSize && chunk.Len() == 0 {
				// Split oversized line into character chunks
				for j := 0; j < len(line); j += maxSize {
					end := j + maxSize
					if end > len(line) {
						end = len(line)
					}
					chunks = append(chunks, CodeChunk{
						Content:    line[j:end],
						ChunkIndex: chunkIndex,
						StartLine:  startLine,
						EndLine:    startLine,
						Header:     buildHeader(filePath, language, ""),
					})
					chunkIndex++
				}
				i++
				break
			}

			// Check if adding this line exceeds limit
			if chunk.Len() > 0 && chunk.Len()+len(line)+1 > maxSize {
				break
			}

			// Hard limit: force split if reaching maxSize
			if chunk.Len() >= maxSize {
				break
			}

			chunk.WriteString(line)
			chunk.WriteString("\n")
			i++
		}

		endLine := i

		// Only add chunk if we actually accumulated content
		if chunk.Len() > 0 {
			chunks = append(chunks, CodeChunk{
				Content:    chunk.String(),
				ChunkIndex: chunkIndex,
				StartLine:  startLine,
				EndLine:    endLine,
				Header:     buildHeader(filePath, language, ""),
			})

			// Backtrack for overlap
			overlapLines := overlap / 50 // Rough: 50 chars per line
			if overlapLines > 0 && i < len(lines) {
				i -= overlapLines
				if i < startLine {
					i = startLine
				}
			}

			chunkIndex++
		}
	}

	return chunks
}

// buildHeader creates a context header for better retrieval
// Format: "path/to/file.go :: Go :: func FunctionName"
func buildHeader(filePath, language, symbol string) string {
	if symbol != "" {
		return fmt.Sprintf("%s :: %s :: %s", filePath, language, symbol)
	}
	return fmt.Sprintf("%s :: %s", filePath, language)
}

// extractSymbol tries to extract the main symbol name from code
func extractSymbol(code string) string {
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Go: func Name or func (r *Receiver) Name
		if strings.HasPrefix(trimmed, "func ") {
			// Remove "func " prefix
			rest := strings.TrimPrefix(trimmed, "func ")

			// Check if it's a method: starts with (
			if strings.HasPrefix(rest, "(") {
				// Find closing ) for receiver
				closeParen := strings.Index(rest, ")")
				if closeParen > 0 && closeParen+1 < len(rest) {
					// Extract method name after receiver
					methodPart := strings.TrimSpace(rest[closeParen+1:])
					parts := strings.Fields(methodPart)
					if len(parts) > 0 {
						// Remove any trailing ( from function signature
						name := parts[0]
						if idx := strings.Index(name, "("); idx > 0 {
							name = name[:idx]
						}
						return "func " + name
					}
				}
			} else {
				// Regular function
				parts := strings.Fields(rest)
				if len(parts) > 0 {
					name := parts[0]
					if idx := strings.Index(name, "("); idx > 0 {
						name = name[:idx]
					}
					return "func " + name
				}
			}
		}

		// Go: type Name
		if strings.HasPrefix(trimmed, "type ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return "type " + parts[1]
			}
		}

		// TypeScript: export class/function/const
		if strings.HasPrefix(trimmed, "export ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				// export default/const/class/function Name
				return parts[1] + " " + parts[2]
			}
		}
	}

	return ""
}

// getOverlapLines returns the last N characters worth of lines in CORRECT order
func getOverlapLines(lines []string, fromIndex, numChars int) string {
	if fromIndex <= 0 {
		return ""
	}

	// Collect lines backwards, then reverse to correct order
	var collected []string
	charsCollected := 0

	for i := fromIndex - 1; i >= 0 && charsCollected < numChars; i-- {
		line := lines[i]
		collected = append(collected, line)
		charsCollected += len(line) + 1 // +1 for newline
	}

	// Reverse to get correct order
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	return strings.Join(collected, "\n") + "\n"
}

// countLines counts the number of newlines in text
func countLines(text string) int {
	return strings.Count(text, "\n") + 1
}

// generateChunkID creates a stable identifier for a chunk
// Format: hash(filePath + startLine + endLine)
func generateChunkID(filePath string, startLine, endLine int) string {
	key := fmt.Sprintf("%s:%d:%d", filePath, startLine, endLine)
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for compact ID
}
