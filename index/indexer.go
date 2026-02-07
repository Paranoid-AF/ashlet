package index

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const indexBatchSize = 32

// SearchResult holds a matching command and its similarity score.
type SearchResult struct {
	Command    string
	Similarity float64
}

// vectorEntry stores an embedded command and its vector.
type vectorEntry struct {
	Command string
	Vector  []float32
}

// Indexer reads and indexes shell history files using in-memory TTL cache.
type Indexer struct {
	historyPath        string // single most-recently-modified history file
	embedder           *Embedder
	dimensions         int
	maxHistoryCommands int
	ttl                time.Duration

	mu          sync.RWMutex
	entries     map[string]vectorEntry // hash -> entry
	lastIndexed time.Time
}

// NewIndexer creates a new history indexer.
// If embedder is nil, semantic features are disabled (RecentCommands still works).
func NewIndexer(embedder *Embedder, dimensions, maxHistoryCommands int, ttl time.Duration) *Indexer {
	return &Indexer{
		historyPath:        resolveHistoryPath(),
		embedder:           embedder,
		dimensions:         dimensions,
		maxHistoryCommands: maxHistoryCommands,
		ttl:                ttl,
		entries:            make(map[string]vectorEntry),
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

	// Batch embed
	var batch []struct {
		hash string
		cmd  string
	}

	for _, cmd := range cmds {
		hash := hashCommand(cmd)
		idx.mu.RLock()
		_, exists := idx.entries[hash]
		idx.mu.RUnlock()
		if exists {
			continue
		}

		batch = append(batch, struct {
			hash string
			cmd  string
		}{hash, cmd})

		if len(batch) >= indexBatchSize {
			if err := idx.embedBatch(batch); err != nil {
				slog.Error("batch embed error", "error", err)
			}
			batch = batch[:0]
		}
	}

	// Flush remaining batch
	if len(batch) > 0 {
		if err := idx.embedBatch(batch); err != nil {
			slog.Error("batch embed error", "error", err)
		}
	}

	idx.mu.Lock()
	idx.lastIndexed = time.Now()
	idx.mu.Unlock()

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

func (idx *Indexer) embedBatch(batch []struct {
	hash string
	cmd  string
}) error {
	// Redact commands before sending to embedding API
	redacted := make([]string, len(batch))
	for i, b := range batch {
		redacted[i] = RedactCommand(b.cmd)
	}

	vectors, err := idx.embedder.EmbedBatch(redacted)
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()
	for i, b := range batch {
		idx.entries[b.hash] = vectorEntry{
			Command: redacted[i],
			Vector:  vectors[i],
		}
	}
	return nil
}

// SearchRelevant embeds the query and returns the topK most similar commands.
// Re-indexes if TTL has expired.
func (idx *Indexer) SearchRelevant(query string, topK int) ([]string, error) {
	if idx.embedder == nil {
		return nil, nil
	}

	// Check TTL and re-index if expired
	idx.mu.RLock()
	needsReindex := idx.lastIndexed.IsZero() || time.Since(idx.lastIndexed) > idx.ttl
	idx.mu.RUnlock()

	if needsReindex {
		if err := idx.IndexHistory(); err != nil {
			slog.Error("re-indexing error", "error", err)
		}
	}

	queryVec, err := idx.embedder.Embed(RedactCommand(query))
	if err != nil {
		return nil, err
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.entries) == 0 || topK <= 0 {
		return nil, nil
	}

	type scored struct {
		command    string
		similarity float64
	}

	queryNorm := vecNorm(queryVec)
	if queryNorm == 0 {
		return nil, nil
	}

	results := make([]scored, 0, len(idx.entries))
	for _, entry := range idx.entries {
		sim := cosineSimilarity(queryVec, entry.Vector, queryNorm)
		results = append(results, scored{command: entry.Command, similarity: sim})
	}

	// Simple selection sort for topK
	if topK > len(results) {
		topK = len(results)
	}
	for i := 0; i < topK; i++ {
		best := i
		for j := i + 1; j < len(results); j++ {
			if results[j].similarity > results[best].similarity {
				best = j
			}
		}
		results[i], results[best] = results[best], results[i]
	}

	commands := make([]string, topK)
	for i := 0; i < topK; i++ {
		commands[i] = results[i].command
	}
	return commands, nil
}

// Close releases resources held by the indexer.
func (idx *Indexer) Close() {
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

func vecNorm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

func cosineSimilarity(a, b []float32, aNorm float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	var bSumSq float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		bSumSq += float64(b[i]) * float64(b[i])
	}
	bNorm := math.Sqrt(bSumSq)
	if aNorm == 0 || bNorm == 0 {
		return 0
	}
	return dot / (aNorm * bNorm)
}
