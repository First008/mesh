package vectorstore

import (
	"math"
	"strings"
)

// FileCandidate represents a file with rich scoring information for hybrid ranking.
type FileCandidate struct {
	BasePath string
	Language string

	// Semantic scores (from vector search chunks)
	BestChunkScore  float32   // Highest chunk score
	AvgChunkScore   float32   // Average of all chunk scores
	TopKChunkScores []float32 // Top-K chunk scores for depth analysis
	ChunkCount      int       // Number of chunks for this file

	// Hybrid scores
	KeywordScore float32 // Keyword matching score (length-normalized)
	PathScore    float32 // File path relevance score
	HybridScore  float32 // Final weighted hybrid score

	// Token tracking
	EstimatedTokens int // Estimated token count for budget management
}

// FileSelection represents either a complete file or top chunks from an oversized file.
type FileSelection struct {
	BasePath        string
	Language        string
	Score           float32
	IsPartial       bool  // true if only top chunks included
	ChunkIndices    []int // which chunks to include (empty = all chunks)
	EstimatedTokens int
}

// extractKeywords performs conservative keyword extraction from a query.
// - Filters tokens by length > 3 to prefer meaningful identifiers
// - Applies English and Go-specific stoplist to avoid common tokens
// - Returns empty slice if no useful keywords found
func extractKeywords(query string) []string {
	// Normalize to lowercase
	query = strings.ToLower(query)

	// Tokenize on non-alphanumeric characters
	words := strings.FieldsFunc(query, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-')
	})

	// Stopwords: Common English + ultra-common Go tokens
	stopWords := map[string]bool{
		// English stop words
		"what": true, "how": true, "where": true, "when": true, "why": true,
		"does": true, "is": true, "are": true, "was": true, "were": true,
		"the": true, "a": true, "an": true, "in": true, "on": true, "at": true,
		"to": true, "for": true, "of": true, "with": true, "by": true, "from": true,
		"this": true, "that": true, "these": true, "those": true, "it": true, "its": true,
		"do": true, "did": true, "can": true, "could": true, "would": true, "should": true,
		"will": true, "shall": true, "may": true, "might": true, "must": true,
		"have": true, "has": true, "had": true, "been": true, "being": true,
		"and": true, "or": true, "but": true, "not": true, "if": true, "then": true,

		// Go common tokens (avoid false boosts)
		"ctx": true, "err": true, "error": true, "nil": true, "bool": true,
		"string": true, "int": true, "int32": true, "int64": true, "uint": true,
		"float": true, "float32": true, "float64": true, "byte": true, "rune": true,
		"func": true, "return": true, "else": true,
		"range": true, "var": true, "const": true, "type": true, "struct": true,
		"interface": true, "map": true, "slice": true, "array": true, "channel": true,
		"new": true, "make": true, "len": true, "cap": true, "append": true,
		"delete": true, "copy": true, "close": true, "defer": true, "go": true,
		"select": true, "case": true, "switch": true, "break": true, "continue": true,
		"package": true, "import": true, "export": true, "default": true,
		"config": true, "logger": true, "client": true, "server": true, // ultra-common
		"context": true, "request": true, "response": true, "handler": true,
		"service": true, "method": true, "function": true, "class": true,
	}

	keywords := []string{}
	for _, word := range words {
		// Prefer longer words (likely meaningful identifiers)
		// len > 3 filters out most noise
		if len(word) > 3 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// calculateKeywordScore scores a file based on keyword occurrences,
// with length normalization to avoid bias toward large files.
//
// Returns 0.0 if no keywords match.
// Score formula: (matches/totalKeywords) * min(log(1 + normalizedOcc)/2.0, 1.0)
func calculateKeywordScore(content string, keywords []string, fileLength int) float32 {
	if len(keywords) == 0 {
		return 0.0 // No keywords to match
	}

	contentLower := strings.ToLower(content)
	matches := 0
	totalOccurrences := 0

	for _, keyword := range keywords {
		count := strings.Count(contentLower, keyword)
		if count > 0 {
			matches++
			totalOccurrences += count
		}
	}

	if matches == 0 {
		return 0.0 // No matches found
	}

	// Normalize by file length to avoid big-file bias
	// Divide by fileLength/10000 (capped at 1.0 minimum to avoid division issues)
	lengthNormalizer := math.Max(float64(fileLength)/10000.0, 1.0)
	normalizedOccurrences := float64(totalOccurrences) / lengthNormalizer

	// Score components:
	// 1. Match ratio: what fraction of keywords were found?
	matchRatio := float32(matches) / float32(len(keywords))

	// 2. Occurrence boost: log-scaled to avoid over-rewarding common terms
	//    Capped at 2.0 to keep normalized score in 0-1 range
	occurrenceBoost := float32(math.Log(1.0 + normalizedOccurrences))

	// Final score: combine ratio and boost, normalize to 0-1
	score := matchRatio * min32(occurrenceBoost/2.0, 1.0)

	return score
}

// calculatePathScore scores a file path based on keyword presence.
// Simple keyword-in-path matching for tie-breaking.
// Returns fraction of keywords found in the path (0.0 to 1.0).
func calculatePathScore(filePath string, keywords []string) float32 {
	if len(keywords) == 0 {
		return 0.0
	}

	pathLower := strings.ToLower(filePath)
	matches := 0

	for _, keyword := range keywords {
		if strings.Contains(pathLower, keyword) {
			matches++
		}
	}

	return float32(matches) / float32(len(keywords))
}

// calculateAggregateScore computes a weighted average of top-K chunk scores
// to measure the depth of relevance (files with multiple relevant chunks rank higher).
//
// Weights: [0.5, 0.3, 0.2] for top-3 chunks
func calculateAggregateScore(topKScores []float32) float32 {
	if len(topKScores) == 0 {
		return 0.0
	}

	// Weights favor top chunks but consider multiple matches
	weights := []float32{0.5, 0.3, 0.2}
	score := float32(0.0)

	for i, s := range topKScores {
		if i >= len(weights) {
			break
		}
		score += s * weights[i]
	}

	return score
}

// min32 returns the minimum of two float32 values.
func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// max32 returns the maximum of two float32 values.
func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
