// Package history provides shell history indexing for ashlet.
// It periodically reads and embeds shell history entries for context-aware completions.
package history

import (
	"bufio"
	"os"
	"path/filepath"
)

// Indexer reads and indexes shell history files.
type Indexer struct {
	historyPaths []string
}

// NewIndexer creates a new history indexer that watches the given history files.
func NewIndexer() *Indexer {
	home, _ := os.UserHomeDir()
	return &Indexer{
		historyPaths: []string{
			filepath.Join(home, ".zsh_history"),
			filepath.Join(home, ".bash_history"),
		},
	}
}

// RecentCommands returns the last n commands from available history files.
func (idx *Indexer) RecentCommands(n int) []string {
	var commands []string
	for _, path := range idx.historyPaths {
		cmds := readLastLines(path, n)
		commands = append(commands, cmds...)
	}
	if len(commands) > n {
		commands = commands[len(commands)-n:]
	}
	return commands
}

func readLastLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

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
