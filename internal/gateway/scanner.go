package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/First008/mesh/internal/vectorstore"
	"github.com/rs/zerolog"
)

// BranchScanner periodically scans all repositories for branch changes
// and triggers incremental re-indexing when commits change
type BranchScanner struct {
	gateway      *Gateway
	scanInterval time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
	logger       zerolog.Logger
}

// NewBranchScanner creates a new periodic branch scanner
func NewBranchScanner(gateway *Gateway, scanInterval time.Duration, logger zerolog.Logger) *BranchScanner {
	if scanInterval == 0 {
		scanInterval = 10 * time.Second // Default: scan every 10 seconds
	}

	return &BranchScanner{
		gateway:      gateway,
		scanInterval: scanInterval,
		stopChan:     make(chan struct{}),
		logger:       logger,
	}
}

// Start begins the periodic scanning loop
func (bs *BranchScanner) Start(ctx context.Context) {
	bs.wg.Add(1)
	go bs.scanLoop(ctx)
	bs.logger.Info().
		Dur("interval", bs.scanInterval).
		Msg("Branch scanner started")
}

// Stop gracefully stops the scanner
func (bs *BranchScanner) Stop() {
	close(bs.stopChan)
	bs.wg.Wait()
	bs.logger.Info().Msg("Branch scanner stopped")
}

// scanLoop is the main scanning loop
func (bs *BranchScanner) scanLoop(ctx context.Context) {
	defer bs.wg.Done()

	ticker := time.NewTicker(bs.scanInterval)
	defer ticker.Stop()

	// Run initial scan immediately
	bs.scanAllRepos(ctx)

	for {
		select {
		case <-ticker.C:
			bs.scanAllRepos(ctx)
		case <-bs.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// scanAllRepos scans all configured repositories for branch changes
func (bs *BranchScanner) scanAllRepos(ctx context.Context) {
	bs.logger.Debug().Msg("Starting periodic branch scan")

	// Get all configured repos
	for _, repoConfig := range bs.gateway.config.Repos {
		bs.scanRepo(ctx, repoConfig)
	}
}

// scanRepo scans a single repository for known indexed branches only
func (bs *BranchScanner) scanRepo(ctx context.Context, repoConfig RepoConfig) {
	repoLogger := bs.logger.With().Str("repo", repoConfig.Name).Logger()

	// Check if it's a git repo
	if !vectorstore.IsGitRepo(repoConfig.Path) {
		repoLogger.Debug().Msg("Not a git repository, skipping")
		return
	}

	// Get known branches from metadata directory
	// Only scan branches that have been indexed before
	knownBranches, err := vectorstore.GetKnownBranches(repoConfig.Name)
	if err != nil {
		repoLogger.Debug().Err(err).Msg("Failed to get known branches")
		return
	}

	if len(knownBranches) == 0 {
		repoLogger.Debug().Msg("No known branches to scan yet")
		return
	}

	repoLogger.Debug().
		Int("known_branches", len(knownBranches)).
		Msg("Scanning known branches for changes")

	// Check each known branch for changes
	for _, branch := range knownBranches {
		bs.checkBranchForChanges(ctx, repoConfig, branch)
	}
}

// checkBranchForChanges checks if a branch needs re-indexing
func (bs *BranchScanner) checkBranchForChanges(ctx context.Context, repoConfig RepoConfig, branch string) {
	repoLogger := bs.logger.With().
		Str("repo", repoConfig.Name).
		Str("branch", branch).
		Logger()

	// Check if this branch needs re-indexing
	needsReindex, currentCommit, err := vectorstore.NeedsReindexing(repoConfig.Path, repoConfig.Name, branch)
	if err != nil {
		repoLogger.Warn().Err(err).Msg("Failed to check reindexing status")
		return
	}

	if !needsReindex {
		repoLogger.Debug().Msg("No changes detected")
		return
	}

	// Load metadata to see old commit
	meta, _ := vectorstore.LoadMetadata(repoConfig.Name, branch)
	oldCommit := "none"
	if meta != nil {
		oldCommit = meta.CommitSHA[:8]
	}

	repoLogger.Info().
		Str("old_commit", oldCommit).
		Str("new_commit", currentCommit[:8]).
		Msg("Branch has changes, triggering re-index")

	// Trigger re-indexing via the gateway's ReindexBranch method
	if err := bs.gateway.ReindexBranch(ctx, repoConfig.Name, branch); err != nil {
		repoLogger.Error().Err(err).Msg("Failed to trigger re-index")
	} else {
		repoLogger.Info().Msg("Re-index triggered successfully")
	}
}
