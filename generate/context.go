package generate

import (
	"context"
	"log/slog"
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
		embeddingEnabled: embeddingEnabled,
		noRawHistory:     noRawHistory,
	}

	if embeddingEnabled {
		go g.historyIndexer.StartRefreshLoop()
	}

	return g
}

// Gather collects context based on the completion request.
func (g *Gatherer) Gather(ctx context.Context, req *ashlet.Request) *Info {
	info := &Info{}

	if g.noRawHistory && g.embeddingEnabled {
		// Block-wait for indexing to complete (up to 10s), then return only relevant commands.
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-g.historyIndexer.InitDone():
			if cmds, err := g.historyIndexer.SearchRelevant(req.Input, 20); err == nil && len(cmds) > 0 {
				info.RelevantCommands = cmds
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
		case <-g.historyIndexer.InitDone():
			if cmds, err := g.historyIndexer.SearchRelevant(req.Input, 20); err == nil && len(cmds) > 0 {
				info.RelevantCommands = cmds
			}
		default:
			// Indexing still in progress, skip semantic search
		}
	}

	return info
}

// LoadIndexCache loads a previously saved embedding cache from disk.
func (g *Gatherer) LoadIndexCache(path string) error {
	model := g.historyIndexer.EmbeddingModel()
	if model == "" {
		return nil
	}
	return g.historyIndexer.LoadCache(path, model)
}

// SaveIndexCache writes the current embedding index to disk.
func (g *Gatherer) SaveIndexCache(path string) error {
	model := g.historyIndexer.EmbeddingModel()
	if model == "" {
		return nil
	}
	return g.historyIndexer.SaveCache(path, model)
}

// Close releases resources held by the gatherer.
func (g *Gatherer) Close() {
	g.historyIndexer.Close()
}
