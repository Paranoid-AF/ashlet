package index

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/hnsw"
)

const indexBatchSize = 32

// Indexer reads and indexes shell history files using in-memory TTL cache.
type Indexer struct {
	historyPath        string // single most-recently-modified history file
	embedder           *Embedder
	maxHistoryCommands int
	ttl                time.Duration

	mu       sync.RWMutex
	graph    *hnsw.Graph[string] // HNSW graph, keyed by command hash
	commands map[string]string   // hash -> redacted command text

	stopCh   chan struct{}
	initDone chan struct{}
	initOnce sync.Once
	closeOnce sync.Once
}

// NewIndexer creates a new history indexer.
// If embedder is nil, semantic features are disabled (RecentCommands still works).
func NewIndexer(embedder *Embedder, maxHistoryCommands int, ttl time.Duration) *Indexer {
	return &Indexer{
		historyPath:        resolveHistoryPath(),
		embedder:           embedder,
		maxHistoryCommands: maxHistoryCommands,
		ttl:                ttl,
		graph:              hnsw.NewGraph[string](),
		commands:           make(map[string]string),
		stopCh:             make(chan struct{}),
		initDone:           make(chan struct{}),
	}
}

// resolveHistoryPath picks the single most recently modified history file.
func resolveHistoryPath() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".zsh_history"),
		filepath.Join(home, ".bash_history"),
	}

	if hf := os.Getenv("HISTFILE"); hf != "" {
		candidates = append([]string{hf}, candidates...)
	}

	var bestPath string
	var bestTime time.Time

	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			bestTime = info.ModTime()
			bestPath = path
		}
	}

	return bestPath
}

// RecentCommands returns the last n commands from the history file.
func (idx *Indexer) RecentCommands(n int) []string {
	if idx.historyPath == "" {
		return nil
	}
	lines := readLastLines(idx.historyPath, n)
	// Parse history format
	cmds := make([]string, 0, len(lines))
	for _, line := range lines {
		cmd := parseHistoryLine(line)
		if cmd != "" {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) > n {
		cmds = cmds[len(cmds)-n:]
	}
	return cmds
}

// IndexHistory reads the last N commands from the history file and embeds them.
func (idx *Indexer) IndexHistory() error {
	if idx.embedder == nil || idx.historyPath == "" {
		return nil
	}

	cmds := idx.readTailCommands()
	if len(cmds) == 0 {
		return nil
	}

	// Collect new commands that need embedding
	idx.mu.RLock()
	var toEmbed []struct {
		hash string
		cmd  string
	}
	for _, cmd := range cmds {
		hash := hashCommand(cmd)
		if _, exists := idx.graph.Lookup(hash); !exists {
			toEmbed = append(toEmbed, struct {
				hash string
				cmd  string
			}{hash, cmd})
		}
	}
	idx.mu.RUnlock()

	if len(toEmbed) == 0 {
		return nil
	}

	// Embed in batches via API, accumulating results locally
	var allNodes []hnsw.Node[string]
	allCommands := make(map[string]string, len(toEmbed))

	for i := 0; i < len(toEmbed); i += indexBatchSize {
		end := i + indexBatchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		redacted := make([]string, len(batch))
		for j, b := range batch {
			redacted[j] = RedactCommand(b.cmd)
		}

		vectors, err := idx.embedder.EmbedBatch(redacted)
		if err != nil {
			slog.Error("batch embed error", "error", err)
			continue
		}

		for j, b := range batch {
			allNodes = append(allNodes, hnsw.MakeNode(b.hash, vectors[j]))
			allCommands[b.hash] = redacted[j]
		}
	}

	// Single graph insertion under one write lock
	if len(allNodes) > 0 {
		idx.mu.Lock()
		idx.graph.Add(allNodes...)
		for k, v := range allCommands {
			idx.commands[k] = v
		}
		idx.mu.Unlock()
	}

	return nil
}

// readTailCommands reads the last maxHistoryCommands from the history file.
func (idx *Indexer) readTailCommands() []string {
	lines := readLastLines(idx.historyPath, idx.maxHistoryCommands)
	cmds := make([]string, 0, len(lines))
	seen := make(map[string]bool)
	for _, line := range lines {
		cmd := parseHistoryLine(line)
		if cmd == "" || seen[cmd] {
			continue
		}
		seen[cmd] = true
		cmds = append(cmds, cmd)
	}
	return cmds
}

// StartRefreshLoop runs IndexHistory immediately, then re-indexes every TTL interval.
// It blocks until Close() is called. If embedder is nil, it closes initDone and returns.
func (idx *Indexer) StartRefreshLoop() {
	if idx.embedder == nil {
		idx.initOnce.Do(func() { close(idx.initDone) })
		return
	}

	if err := idx.IndexHistory(); err != nil {
		slog.Error("initial indexing error", "error", err)
	}
	idx.initOnce.Do(func() { close(idx.initDone) })

	ticker := time.NewTicker(idx.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-idx.stopCh:
			return
		case <-ticker.C:
			if err := idx.IndexHistory(); err != nil {
				slog.Error("periodic re-indexing error", "error", err)
			}
		}
	}
}

// InitDone returns a channel that is closed after the first IndexHistory call completes.
func (idx *Indexer) InitDone() <-chan struct{} {
	return idx.initDone
}

// SearchRelevant embeds the query and returns the topK most similar commands.
func (idx *Indexer) SearchRelevant(query string, topK int) ([]string, error) {
	if idx.embedder == nil {
		return nil, nil
	}

	queryVec, err := idx.embedder.Embed(RedactCommand(query))
	if err != nil {
		return nil, err
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.graph.Len() == 0 || topK <= 0 {
		return nil, nil
	}

	neighbors := idx.graph.Search(queryVec, topK)
	commands := make([]string, len(neighbors))
	for i, n := range neighbors {
		commands[i] = idx.commands[n.Key]
	}
	return commands, nil
}

// Close stops the refresh loop and releases resources held by the indexer.
func (idx *Indexer) Close() {
	idx.closeOnce.Do(func() {
		close(idx.stopCh)
	})
	if idx.embedder != nil {
		idx.embedder.Close()
	}
}

// parseHistoryLine strips shell-specific prefixes from history lines.
// Zsh extended history format: ": 1234567890:0;actual command"
// Bash format: just the command (no prefix)
func parseHistoryLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	// Zsh extended history: ": <timestamp>:<duration>;<command>"
	if strings.HasPrefix(line, ": ") {
		if idx := strings.Index(line, ";"); idx != -1 {
			return strings.TrimSpace(line[idx+1:])
		}
	}
	return line
}

func hashCommand(cmd string) string {
	h := sha256.Sum256([]byte(cmd))
	return fmt.Sprintf("%x", h)
}

func readLastLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// For efficiency, seek near end of file for large files
	info, err := f.Stat()
	if err != nil {
		return nil
	}

	// Estimate: average 50 bytes per line, read 2x to be safe
	estimatedBytes := int64(n) * 100
	if estimatedBytes < info.Size() {
		if _, err := f.Seek(-estimatedBytes, io.SeekEnd); err == nil {
			// Skip partial first line
			reader := bufio.NewReader(f)
			reader.ReadString('\n')
			var lines []string
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if len(lines) >= n {
				return lines[len(lines)-n:]
			}
			// Not enough lines, fall through to full read
		}
		f.Seek(0, io.SeekStart)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
