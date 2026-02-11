package index

import (
	"encoding/json"
	"os"

	"github.com/coder/hnsw"
)

type cacheFile struct {
	Model   string       `json:"model"`
	Entries []cacheEntry `json:"entries"`
}

type cacheEntry struct {
	Hash      string    `json:"hash"`
	Command   string    `json:"command"`
	Embedding []float32 `json:"embedding"`
}

// EmbeddingModel returns the model name used by the embedder, or empty if disabled.
func (idx *Indexer) EmbeddingModel() string {
	if idx.embedder == nil {
		return ""
	}
	return idx.embedder.Model()
}

// SaveCache writes the current index (commands + embeddings) to disk.
func (idx *Indexer) SaveCache(path string, model string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entries := make([]cacheEntry, 0, len(idx.commands))
	for hash, cmd := range idx.commands {
		vec, ok := idx.graph.Lookup(hash)
		if !ok {
			continue
		}
		entries = append(entries, cacheEntry{
			Hash:      hash,
			Command:   cmd,
			Embedding: vec,
		})
	}

	data, err := json.Marshal(cacheFile{
		Model:   model,
		Entries: entries,
	})
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadCache loads a previously saved index from disk.
// If the model doesn't match, the cache is silently skipped.
func (idx *Indexer) LoadCache(path string, model string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return err
	}

	if cf.Model != model {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	nodes := make([]hnsw.Node[string], 0, len(cf.Entries))
	for _, e := range cf.Entries {
		nodes = append(nodes, hnsw.MakeNode(e.Hash, e.Embedding))
		idx.commands[e.Hash] = e.Command
	}

	if len(nodes) > 0 {
		idx.graph.Add(nodes...)
		// Mark init as done so searches can use cached data immediately,
		// without waiting for the refresh loop's first IndexHistory() call.
		idx.initOnce.Do(func() { close(idx.initDone) })
	}

	return nil
}
