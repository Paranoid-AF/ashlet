package generate

import (
	"context"
	"log/slog"
	"sync"
	"time"

	ashlet "github.com/Paranoid-AF/ashlet"
	"github.com/Paranoid-AF/ashlet/index"
)

// Info holds gathered context for a completion request.
type Info struct {
	RecentCommands   []string
	RelevantCommands []string
}

// Gatherer collects context for completion requests.
type Gatherer struct {
	historyIndexer   *index.Indexer
	indexOnce        sync.Once
	indexDone        chan struct{}
	indexErr         error
	embeddingEnabled bool
	noRawHistory     bool
}

// NewGatherer creates a new context gatherer.
// embedder may be nil to disable semantic features.
func NewGatherer(embedder *index.Embedder, cfg *ashlet.Config) *Gatherer {
	var maxHistory int
	var ttlMinutes int
	var noRawHistory bool
	embeddingEnabled := embedder != nil
	if cfg != nil {
		maxHistory = cfg.Embedding.MaxHistoryCommands
		ttlMinutes = cfg.Embedding.TTLMinutes
		if cfg.Generation.NoRawHistory != nil {
			noRawHistory = *cfg.Generation.NoRawHistory
		}
	}
	if maxHistory == 0 {
		maxHistory = 3000
	}
	if ttlMinutes == 0 {
		ttlMinutes = 60
	}

	g := &Gatherer{
		historyIndexer:   index.NewIndexer(embedder, maxHistory, time.Duration(ttlMinutes)*time.Minute),
		indexDone:        make(chan struct{}),
		embeddingEnabled: embeddingEnabled,
		noRawHistory:     noRawHistory,
	}

	// Eagerly start indexing when embedding is enabled so history is
	// ready by the time the first completion request arrives. This is
	// especially important after a config reload that enables embedding.
	if embeddingEnabled {
		g.startIndexing()
	}

	return g
}

// startIndexing kicks off background history indexing exactly once.
func (g *Gatherer) startIndexing() {
	g.indexOnce.Do(func() {
		go func() {
			defer close(g.indexDone)
			if err := g.historyIndexer.IndexHistory(); err != nil {
				g.indexErr = err
				slog.Error("background indexing error", "error", err)
			}
		}()
	})
}

// Gather collects context based on the completion request.
func (g *Gatherer) Gather(ctx context.Context, req *ashlet.Request) *Info {
	// Ensure indexing has been triggered (no-op if already started)
	g.startIndexing()

	info := &Info{}

	if g.noRawHistory && g.embeddingEnabled {
		// Block-wait for indexing to complete (up to 10s), then return only relevant commands.
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-g.indexDone:
			if g.indexErr == nil {
				if cmds, err := g.historyIndexer.SearchRelevant(req.Input, 5); err == nil && len(cmds) > 0 {
					info.RelevantCommands = cmds
				}
			} else {
				slog.Warn("embedding indexing failed, no history context available")
			}
		case <-timer.C:
			slog.Warn("embedding indexing timed out, no history context available")
		case <-ctx.Done():
			// Request cancelled
		}
		return info
	}

	// Default: include recent commands
	info.RecentCommands = g.historyIndexer.RecentCommands(20)

	if g.embeddingEnabled {
		// Non-blocking semantic search if indexing has completed
		select {
		case <-g.indexDone:
			if cmds, err := g.historyIndexer.SearchRelevant(req.Input, 5); err == nil && len(cmds) > 0 {
				info.RelevantCommands = cmds
			}
		default:
			// Indexing still in progress, skip semantic search
		}
	}

	return info
}

// Close releases resources held by the gatherer.
func (g *Gatherer) Close() {
	g.historyIndexer.Close()
}
