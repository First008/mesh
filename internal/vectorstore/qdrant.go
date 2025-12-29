package vectorstore

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/First008/mesh/internal/filetypes"
	"github.com/qdrant/go-client/qdrant"
	"github.com/rs/zerolog"
)

// QdrantStore implements VectorStore using Qdrant vector database
type QdrantStore struct {
	client            *qdrant.Client
	embeddingProvider EmbeddingProvider
	collectionName    string
	logger            zerolog.Logger
	searchConfig      *SearchConfig // Configuration for smart file selection
}

// NewQdrantStore creates a new Qdrant vector store with an embedding provider
// For backward compatibility, uses "main" as default branch if branch is empty
func NewQdrantStore(qdrantURL string, embeddingProvider EmbeddingProvider, repoName string, logger zerolog.Logger) (*QdrantStore, error) {
	return NewQdrantStoreWithBranch(qdrantURL, embeddingProvider, repoName, "main", logger)
}

// NewQdrantStoreWithBranch creates a new Qdrant vector store with branch support
// Collection name format: mesh-{repo}-{branch}-v1
func NewQdrantStoreWithBranch(qdrantURL string, embeddingProvider EmbeddingProvider, repoName, branch string, logger zerolog.Logger) (*QdrantStore, error) {
	if qdrantURL == "" {
		return nil, fmt.Errorf("qdrant URL is required")
	}

	if embeddingProvider == nil {
		return nil, fmt.Errorf("embedding provider is required")
	}

	// Parse host and port from URL
	// Format: "localhost:6334" or just "localhost" (defaults to 6334)
	host, port := parseQdrantURL(qdrantURL)

	// Create Qdrant client
	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	// Collection name: mesh-{repo-name}-{branch}-v1
	// Sanitize branch name for collection (replace / with -)
	safeBranch := SanitizeBranchName(branch)
	collectionName := fmt.Sprintf("mesh-%s-%s-v1", repoName, safeBranch)

	store := &QdrantStore{
		client:            qdrantClient,
		embeddingProvider: embeddingProvider,
		collectionName:    collectionName,
		logger:            logger,
		searchConfig:      DefaultSearchConfig(), // Use default smart search config
	}

	// Ensure collection exists
	if err := store.ensureCollection(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	logger.Info().
		Str("collection", collectionName).
		Str("qdrant_url", qdrantURL).
		Msg("Qdrant store initialized")

	return store, nil
}

// ensureCollection creates the collection if it doesn't exist
func (qs *QdrantStore) ensureCollection(ctx context.Context) error {
	// Check if collection exists
	exists, err := qs.client.CollectionExists(ctx, qs.collectionName)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}

	if exists {
		qs.logger.Debug().Str("collection", qs.collectionName).Msg("Collection already exists")
		return nil
	}

	// Create collection with vector configuration
	// Use dimension from embedding provider
	vectorDim := uint64(qs.embeddingProvider.GetDimensions())

	// Optimized HNSW parameters for code RAG
	// M=16: Good balance between speed and recall for code search
	// EfConstruct=128: Build quality (higher = better index but slower build)
	m := uint64(16)
	efConstruct := uint64(128)

	err = qs.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: qs.collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     vectorDim,
			Distance: qdrant.Distance_Cosine,
			HnswConfig: &qdrant.HnswConfigDiff{
				M:                 &m,
				EfConstruct:       &efConstruct,
				FullScanThreshold: nil, // Use default
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	qs.logger.Info().Str("collection", qs.collectionName).Msg("Collection created")
	return nil
}

// IndexFile indexes a file by creating an embedding and storing it in Qdrant
func (qs *QdrantStore) IndexFile(ctx context.Context, filePath, content string) error {
	// Create file hash for change detection
	fileHash := computeHash(content)

	// Create embedding
	embedding, err := qs.createEmbedding(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to create embedding: %w", err)
	}

	// Generate deterministic UUID from file path
	pointID := generatePointID(filePath)

	// Extract base path (without #chunkN suffix) for deletion
	basePath := strings.Split(filePath, "#")[0]

	// Upsert to Qdrant
	_, err = qs.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: qs.collectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDNum(pointID),
				Vectors: qdrant.NewVectors(embedding...),
				Payload: qdrant.NewValueMap(map[string]any{
					"file_path":   filePath,
					"base_path":   basePath, // For deleting all chunks of a file
					"content":     content,
					"file_hash":   fileHash,
					"language":    detectLanguage(filePath),
					"chunk_index": extractChunkIndex(filePath), // For ordering chunks during reconstruction
				}),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	qs.logger.Debug().
		Str("file_path", filePath).
		Str("file_hash", fileHash).
		Msg("File indexed")

	return nil
}

// Search performs semantic search for relevant code
func (qs *QdrantStore) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	qs.logger.Info().
		Str("query", truncate(query, 60)).
		Int("limit", limit).
		Msg("Starting vector search")

	// Create query embedding
	embedding, err := qs.createEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to create query embedding: %w", err)
	}

	// Log first few values to verify embedding is valid
	embeddingSample := embedding[:5]
	qs.logger.Info().
		Int("embedding_dim", len(embedding)).
		Floats32("sample_values", embeddingSample).
		Str("collection", qs.collectionName).
		Msg("Query embedding created")

	// Search Qdrant
	// Note: Qdrant automatically uses full scan when HNSW index isn't built yet
	qs.logger.Info().
		Str("collection", qs.collectionName).
		Int("limit", limit).
		Msg("Querying Qdrant")

	searchResult, err := qs.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: qs.collectionName,
		Query:          qdrant.NewQuery(embedding...),
		Limit:          uintPtr(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	qs.logger.Info().
		Int("qdrant_results", len(searchResult)).
		Msg("Qdrant query returned")

	// Convert to SearchResult
	var results []SearchResult
	for _, point := range searchResult {
		payload := point.Payload

		results = append(results, SearchResult{
			FilePath: getStringValue(payload, "file_path"),
			Content:  getStringValue(payload, "content"),
			Score:    point.Score,
			Language: getStringValue(payload, "language"),
			FileHash: getStringValue(payload, "file_hash"),
		})
	}

	qs.logger.Info().
		Int("result_count", len(results)).
		Str("query", truncate(query, 50)).
		Msg("Vector search completed")

	return results, nil
}

// FetchAllChunks retrieves all chunks for a file using base_path filter
func (qs *QdrantStore) FetchAllChunks(ctx context.Context, basePath string) ([]SearchResult, error) {
	// Use Scroll API with filter on base_path to get all chunks
	scrollResult, err := qs.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: qs.collectionName,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				{
					ConditionOneOf: &qdrant.Condition_Field{
						Field: &qdrant.FieldCondition{
							Key: "base_path",
							Match: &qdrant.Match{
								MatchValue: &qdrant.Match_Text{
									Text: basePath,
								},
							},
						},
					},
				},
			},
		},
		WithPayload: qdrant.NewWithPayload(true),
		Limit:       uint32Ptr(100), // Most files won't have more than 100 chunks
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant scroll failed: %w", err)
	}

	var results []SearchResult
	for _, point := range scrollResult {
		payload := point.Payload
		results = append(results, SearchResult{
			FilePath: getStringValue(payload, "file_path"),
			Content:  getStringValue(payload, "content"),
			Score:    0, // Not a search result, no score
			Language: getStringValue(payload, "language"),
			FileHash: getStringValue(payload, "file_hash"),
		})
	}

	return results, nil
}

// Helper methods for smart file selection with adaptive scoring

// buildFileCandidates groups chunks by file and builds rich scoring information
func (qs *QdrantStore) buildFileCandidates(ctx context.Context, rawResults []SearchResult, keywords []string) []*FileCandidate {
	fileMap := make(map[string]*FileCandidate)

	// Group chunks by base path and collect scores
	for _, r := range rawResults {
		basePath := extractBasePath(r.FilePath)

		candidate, exists := fileMap[basePath]
		if !exists {
			candidate = &FileCandidate{
				BasePath:        basePath,
				Language:        r.Language,
				BestChunkScore:  r.Score,
				TopKChunkScores: []float32{r.Score},
				ChunkCount:      1,
				EstimatedTokens: estimateTokens(r.Content),
			}
			fileMap[basePath] = candidate
		} else {
			// Update existing candidate
			candidate.ChunkCount++
			candidate.EstimatedTokens += estimateTokens(r.Content)

			if r.Score > candidate.BestChunkScore {
				candidate.BestChunkScore = r.Score
			}

			// Keep top-3 chunk scores for aggregate scoring
			candidate.TopKChunkScores = append(candidate.TopKChunkScores, r.Score)
			if len(candidate.TopKChunkScores) > 3 {
				// Sort and keep top-3
				scores := candidate.TopKChunkScores
				for i := 0; i < len(scores)-1; i++ {
					for j := i + 1; j < len(scores); j++ {
						if scores[j] > scores[i] {
							scores[i], scores[j] = scores[j], scores[i]
						}
					}
				}
				candidate.TopKChunkScores = scores[:3]
			}
		}
	}

	// Calculate additional scores for each candidate
	candidates := make([]*FileCandidate, 0, len(fileMap))
	for basePath, candidate := range fileMap {
		// Fetch full content for keyword and path scoring
		// Note: This is somewhat inefficient as we're fetching content again
		// but necessary for accurate keyword scoring. Could be optimized later.
		chunks, err := qs.FetchAllChunks(ctx, basePath)
		if err != nil {
			qs.logger.Warn().Err(err).Str("file", basePath).Msg("Failed to fetch chunks for scoring")
			continue
		}

		// Reconstruct full content
		var contentBuilder strings.Builder
		for i, chunk := range chunks {
			if i > 0 {
				contentBuilder.WriteString("\n")
			}
			contentBuilder.WriteString(chunk.Content)
		}
		fullContent := contentBuilder.String()

		// Calculate scores
		candidate.KeywordScore = calculateKeywordScore(fullContent, keywords, len(fullContent))
		candidate.PathScore = calculatePathScore(basePath, keywords)

		// Calculate average chunk score
		avgScore := float32(0.0)
		for _, s := range candidate.TopKChunkScores {
			avgScore += s
		}
		candidate.AvgChunkScore = avgScore / float32(len(candidate.TopKChunkScores))

		candidates = append(candidates, candidate)
	}

	return candidates
}

// calculateAdaptiveThreshold computes a distribution-based threshold with min survivors guard
func (qs *QdrantStore) calculateAdaptiveThreshold(candidates []*FileCandidate, config *SearchConfig) float32 {
	if len(candidates) == 0 {
		return config.MinAbsoluteScore
	}

	// Collect all best chunk scores
	scores := make([]float32, len(candidates))
	for i, c := range candidates {
		scores[i] = c.BestChunkScore
	}

	// Sort ascending for percentile calculation
	sort.Slice(scores, func(i, j int) bool {
		return scores[i] < scores[j]
	})

	// Calculate p90 (90th percentile)
	percentileIdx := int(float32(len(scores)) * config.ScoreDistributionPercentile)
	if percentileIdx >= len(scores) {
		percentileIdx = len(scores) - 1
	}
	distributionThreshold := scores[percentileIdx]

	// Apply floor
	adaptiveThreshold := max32(distributionThreshold, config.MinAbsoluteScore)

	// Count survivors
	survivors := 0
	for _, s := range scores {
		if s >= adaptiveThreshold {
			survivors++
		}
	}

	// Safety: ensure minimum survivors (relax threshold if needed)
	if survivors < config.MinFilesAfterThreshold && len(scores) >= config.MinFilesAfterThreshold {
		// Take Nth highest score where N = MinFilesAfterThreshold
		adaptiveThreshold = scores[len(scores)-config.MinFilesAfterThreshold]
		survivors = config.MinFilesAfterThreshold

		qs.logger.Info().
			Float32("original_threshold", max32(distributionThreshold, config.MinAbsoluteScore)).
			Float32("relaxed_threshold", adaptiveThreshold).
			Int("min_files", config.MinFilesAfterThreshold).
			Msg("Relaxed threshold to ensure min survivors")
	}

	qs.logger.Info().
		Float32("p90_score", distributionThreshold).
		Float32("adaptive_threshold", adaptiveThreshold).
		Int("survivors", survivors).
		Int("total_candidates", len(candidates)).
		Msg("Calculated adaptive threshold")

	return adaptiveThreshold
}

// applyHybridScoring computes weighted hybrid scores with dynamic weight adjustment
func (qs *QdrantStore) applyHybridScoring(candidates []*FileCandidate, keywords []string, config *SearchConfig) {
	// Check if keywords are useful
	avgKeywordScore := float32(0.0)
	for _, c := range candidates {
		avgKeywordScore += c.KeywordScore
	}
	if len(candidates) > 0 {
		avgKeywordScore /= float32(len(candidates))
	}

	// Dynamic weight adjustment: if keywords weak, shift to semantic
	semanticWeight := config.SemanticWeight
	keywordWeight := config.KeywordWeight
	pathWeight := config.PathWeight
	aggregateWeight := config.AggregateWeight

	if len(keywords) == 0 || avgKeywordScore < 0.05 {
		// Keywords not useful - shift weight to semantic
		semanticWeight += keywordWeight
		keywordWeight = 0.0

		qs.logger.Info().
			Int("keyword_count", len(keywords)).
			Float32("avg_keyword_score", avgKeywordScore).
			Msg("No useful keywords found, using semantic-only scoring")
	}

	// Calculate hybrid scores
	for _, candidate := range candidates {
		// Calculate aggregate score from top-K chunks
		aggregateScore := calculateAggregateScore(candidate.TopKChunkScores)

		// Weighted hybrid score
		candidate.HybridScore =
			candidate.BestChunkScore*semanticWeight +
				candidate.KeywordScore*keywordWeight +
				candidate.PathScore*pathWeight +
				aggregateScore*aggregateWeight

		// Penalties for non-code files to prefer actual implementation
		baseLower := strings.ToLower(candidate.BasePath)

		// Heavy penalty for workflow/CI files (rarely useful for code questions)
		if strings.Contains(baseLower, ".github/workflows") ||
			strings.Contains(baseLower, "bitbucket-pipelines") ||
			strings.HasSuffix(baseLower, ".gitlab-ci.yml") {
			candidate.HybridScore *= 0.50 // 50% penalty
			qs.logger.Debug().
				Str("file", candidate.BasePath).
				Msg("Applied workflow file penalty")
		}

		// Penalty for large markdown docs (prefer code for implementation questions)
		if strings.HasSuffix(baseLower, ".md") {
			if candidate.EstimatedTokens > 10000 { // ~40KB file
				candidate.HybridScore *= 0.70 // 30% penalty
				qs.logger.Debug().
					Str("file", candidate.BasePath).
					Int("tokens", candidate.EstimatedTokens).
					Msg("Applied large markdown penalty")
			}
		}

		// Penalty for package-lock.json (huge, low signal)
		if strings.Contains(baseLower, "package-lock.json") ||
			strings.Contains(baseLower, "yarn.lock") {
			candidate.HybridScore *= 0.40 // 60% penalty
			qs.logger.Debug().
				Str("file", candidate.BasePath).
				Msg("Applied lock file penalty")
		}
	}
}

// selectFilesWithinBudget selects files within token budget with oversize fallback
func (qs *QdrantStore) selectFilesWithinBudget(candidates []*FileCandidate, config *SearchConfig) []*FileSelection {
	// Sort by hybrid score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].HybridScore > candidates[j].HybridScore
	})

	selected := []*FileSelection{}
	cumulativeTokens := 0
	maxTokens := config.EffectiveTokenBudget()

	for _, candidate := range candidates {
		// Check file limit first
		if len(selected) >= config.MaxFilesLimit {
			qs.logger.Info().
				Int("max_files", config.MaxFilesLimit).
				Msg("Reached maximum file limit")
			break
		}

		remainingBudget := maxTokens - cumulativeTokens

		// Case 1: File fits completely
		if candidate.EstimatedTokens <= remainingBudget {
			selected = append(selected, &FileSelection{
				BasePath:        candidate.BasePath,
				Language:        candidate.Language,
				Score:           candidate.HybridScore,
				IsPartial:       false,
				ChunkIndices:    nil, // Include all chunks
				EstimatedTokens: candidate.EstimatedTokens,
			})
			cumulativeTokens += candidate.EstimatedTokens
			continue
		}

		// Case 2: File oversized → try top-K chunks fallback
		if candidate.ChunkCount > 0 {
			topKChunks := config.OversizeChunkLimit
			if topKChunks > candidate.ChunkCount {
				topKChunks = candidate.ChunkCount
			}
			estimatedTopKTokens := (candidate.EstimatedTokens / candidate.ChunkCount) * topKChunks

			if estimatedTopKTokens <= remainingBudget && topKChunks > 0 {
				// Include top-scoring chunks only
				// We'll identify which chunks in reconstructFiles
				selected = append(selected, &FileSelection{
					BasePath:        candidate.BasePath,
					Language:        candidate.Language,
					Score:           candidate.HybridScore,
					IsPartial:       true,
					ChunkIndices:    nil, // Will be populated by getTopChunkIndices
					EstimatedTokens: estimatedTopKTokens,
				})
				cumulativeTokens += estimatedTopKTokens

				qs.logger.Info().
					Str("file", candidate.BasePath).
					Int("total_chunks", candidate.ChunkCount).
					Int("included_chunks", topKChunks).
					Int("estimated_tokens", estimatedTopKTokens).
					Msg("Including top chunks from oversized file")
				continue
			}
		}

		// Case 3: Even top-K chunks don't fit → skip
		qs.logger.Warn().
			Str("file", candidate.BasePath).
			Int("remaining_budget", remainingBudget).
			Int("file_tokens", candidate.EstimatedTokens).
			Msg("Skipping file - insufficient budget even for top chunks")
	}

	qs.logger.Info().
		Int("selected_files", len(selected)).
		Int("total_tokens", cumulativeTokens).
		Int("budget", maxTokens).
		Msg("File selection complete")

	return selected
}

// reconstructFiles fetches and reconstructs files based on selections
func (qs *QdrantStore) reconstructFiles(ctx context.Context, selections []*FileSelection) []SearchResult {
	results := []SearchResult{}

	for _, selection := range selections {
		chunks, err := qs.FetchAllChunks(ctx, selection.BasePath)
		if err != nil {
			qs.logger.Warn().Err(err).Str("file", selection.BasePath).Msg("Failed to fetch chunks")
			continue
		}

		if len(chunks) == 0 {
			continue
		}

		// Sort chunks by index
		sort.Slice(chunks, func(i, j int) bool {
			return extractChunkIndex(chunks[i].FilePath) < extractChunkIndex(chunks[j].FilePath)
		})

		// If partial file, select top-K chunks by score
		if selection.IsPartial {
			// Re-score chunks and select top-K
			type chunkWithScore struct {
				chunk SearchResult
				index int
			}
			scoredChunks := make([]chunkWithScore, len(chunks))
			for i, chunk := range chunks {
				scoredChunks[i] = chunkWithScore{chunk: chunk, index: i}
			}

			// Sort by score (using actual scores from vector search)
			// Note: We're approximating here - ideally we'd store chunk scores from initial search
			// For now, just take first K chunks as a simplification
			topK := qs.searchConfig.OversizeChunkLimit
			if topK > len(chunks) {
				topK = len(chunks)
			}
			chunks = chunks[:topK]
		}

		// Concatenate chunk contents
		var contentBuilder strings.Builder
		for i, chunk := range chunks {
			if i > 0 {
				contentBuilder.WriteString("\n")
			}
			contentBuilder.WriteString(chunk.Content)
		}

		results = append(results, SearchResult{
			FilePath: selection.BasePath,
			Content:  contentBuilder.String(),
			Score:    selection.Score,
			Language: selection.Language,
			FileHash: chunks[0].FileHash, // Use first chunk's hash
		})
	}

	return results
}

// SearchWithAggregation searches and reconstructs complete files from chunks
// Uses adaptive scoring, token budget management, and hybrid ranking for improved recall
func (qs *QdrantStore) SearchWithAggregation(ctx context.Context, query string, maxFiles int) ([]SearchResult, error) {
	config := qs.searchConfig

	qs.logger.Info().
		Str("query", truncate(query, 80)).
		Int("max_files", maxFiles).
		Msg("Starting aggregated search with smart file selection")

	// 1. Initial vector search
	rawResults, err := qs.Search(ctx, query, config.InitialChunkLimit)
	if err != nil {
		return nil, err
	}

	if len(rawResults) == 0 {
		qs.logger.Warn().Msg("Vector search returned 0 results")
		return nil, nil
	}

	qs.logger.Info().
		Int("raw_chunks", len(rawResults)).
		Float32("top_score", rawResults[0].Score).
		Msg("Raw search results")

	// 2. Extract keywords (conservative)
	keywords := extractKeywords(query)
	qs.logger.Info().Strs("keywords", keywords).Int("count", len(keywords)).Msg("Extracted keywords")

	// 3. Build file candidates with rich scoring
	candidates := qs.buildFileCandidates(ctx, rawResults, keywords)
	if len(candidates) == 0 {
		qs.logger.Warn().Msg("No file candidates after building")
		return nil, nil
	}

	qs.logger.Info().Int("candidates", len(candidates)).Msg("Built file candidates")

	// 4. Calculate adaptive threshold (p90 + min survivors)
	threshold := qs.calculateAdaptiveThreshold(candidates, config)

	// 5. Filter by adaptive threshold
	filtered := []*FileCandidate{}
	for _, c := range candidates {
		if c.BestChunkScore >= threshold {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) == 0 {
		qs.logger.Warn().
			Float32("threshold", threshold).
			Int("candidates", len(candidates)).
			Msg("No candidates passed adaptive threshold")
		return nil, nil
	}

	qs.logger.Info().
		Int("filtered", len(filtered)).
		Int("total", len(candidates)).
		Msg("Candidates after threshold filtering")

	// 6. Apply hybrid scoring (with dynamic weight adjustment)
	qs.applyHybridScoring(filtered, keywords, config)

	// 7. Select files within token budget (with oversize fallback)
	selections := qs.selectFilesWithinBudget(filtered, config)
	if len(selections) == 0 {
		qs.logger.Warn().Msg("No files selected after token budget")
		return nil, nil
	}

	// 8. Reconstruct files (complete or partial)
	results := qs.reconstructFiles(ctx, selections)

	// Log final results
	fileInfo := make([]string, len(results))
	for i, r := range results {
		fileInfo[i] = fmt.Sprintf("%s (score=%.3f, %d bytes)", r.FilePath, r.Score, len(r.Content))
	}
	qs.logger.Info().
		Int("final_files", len(results)).
		Strs("files", fileInfo).
		Msg("Aggregated search completed")

	return results, nil
}

// DeleteFile removes a file and all its chunks from the index
func (qs *QdrantStore) DeleteFile(ctx context.Context, filePath string) error {
	// Delete all points where base_path matches filePath
	// This handles both single files and all chunks (file#chunk0, file#chunk1, etc.)
	_, err := qs.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: qs.collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{
						{
							ConditionOneOf: &qdrant.Condition_Field{
								Field: &qdrant.FieldCondition{
									Key: "base_path",
									Match: &qdrant.Match{
										MatchValue: &qdrant.Match_Text{
											Text: filePath,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete file and chunks: %w", err)
	}

	qs.logger.Debug().Str("file_path", filePath).Msg("File and all chunks deleted from index")
	return nil
}

// DeleteCollection removes the entire collection
func (qs *QdrantStore) DeleteCollection(ctx context.Context) error {
	err := qs.client.DeleteCollection(ctx, qs.collectionName)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	qs.logger.Info().Str("collection", qs.collectionName).Msg("Collection deleted")
	return nil
}

// GetStats returns statistics about the vector store
func (qs *QdrantStore) GetStats(ctx context.Context) (*Stats, error) {
	info, err := qs.client.GetCollectionInfo(ctx, qs.collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	return &Stats{
		TotalVectors:   int64(info.GetIndexedVectorsCount()),
		CollectionName: qs.collectionName,
		IndexedFiles:   int(info.GetPointsCount()),
	}, nil
}

// Close closes the Qdrant connection
func (qs *QdrantStore) Close() error {
	if qs.client != nil {
		return qs.client.Close()
	}
	return nil
}

// createEmbedding creates an embedding using the configured provider
func (qs *QdrantStore) createEmbedding(ctx context.Context, text string) ([]float32, error) {
	return qs.embeddingProvider.CreateEmbedding(ctx, text)
}

// Helper functions

func generatePointID(filePath string) uint64 {
	// Generate deterministic ID from file path using SHA256
	// Take first 8 bytes as uint64 to avoid collisions
	hash := sha256.Sum256([]byte(filePath))

	// Convert first 8 bytes to uint64
	id := uint64(hash[0]) | uint64(hash[1])<<8 | uint64(hash[2])<<16 | uint64(hash[3])<<24 |
		uint64(hash[4])<<32 | uint64(hash[5])<<40 | uint64(hash[6])<<48 | uint64(hash[7])<<56

	return id
}

func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// detectLanguage returns the language for a file path
// Delegates to filetypes package (single source of truth)
func detectLanguage(filePath string) string {
	lang := filetypes.GetLanguage(filePath)
	if lang == "" {
		return "text"
	}
	return lang
}

func getStringValue(payload map[string]*qdrant.Value, key string) string {
	if val, ok := payload[key]; ok {
		if val.GetStringValue() != "" {
			return val.GetStringValue()
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractChunkIndex parses chunk index from file path suffix
// "file.go#chunk2" -> 2, "file.go" -> 0
func extractChunkIndex(filePath string) int {
	idx := strings.Index(filePath, "#chunk")
	if idx < 0 {
		return 0
	}
	var chunkNum int
	fmt.Sscanf(filePath[idx:], "#chunk%d", &chunkNum)
	return chunkNum
}

// extractBasePath returns the base file path without chunk suffix
// "file.go#chunk2" -> "file.go"
func extractBasePath(filePath string) string {
	if idx := strings.Index(filePath, "#chunk"); idx > 0 {
		return filePath[:idx]
	}
	return filePath
}

func uintPtr(u uint64) *uint64 {
	return &u
}

func uint32Ptr(u uint32) *uint32 {
	return &u
}

func parseQdrantURL(url string) (host string, port int) {
	// Default port for Qdrant gRPC
	port = 6334

	// Check if URL contains port
	parts := strings.Split(url, ":")
	if len(parts) == 2 {
		host = parts[0]
		// Try to parse port
		var err error
		_, err = fmt.Sscanf(parts[1], "%d", &port)
		if err != nil {
			// Invalid port, use default
			port = 6334
		}
	} else {
		host = url
	}

	// Handle common cases
	if host == "" {
		host = "localhost"
	}

	return host, port
}
