package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/hnsw"
)

func TestParseHistoryLineZsh(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{": 1234567890:0;git status", "git status"},
		{": 1234567890:0;ls -la /tmp", "ls -la /tmp"},
		{": 1234567890:0;", ""},
	}
	for _, tt := range tests {
		got := parseHistoryLine(tt.input)
		if got != tt.expected {
			t.Errorf("parseHistoryLine(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseHistoryLineBash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"git status", "git status"},
		{"ls -la /tmp", "ls -la /tmp"},
		{"  git commit -m 'test'  ", "git commit -m 'test'"},
		{"", ""},
	}
	for _, tt := range tests {
		got := parseHistoryLine(tt.input)
		if got != tt.expected {
			t.Errorf("parseHistoryLine(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRecentCommandsReadsBashHistory(t *testing.T) {
	dir := t.TempDir()
	bashHist := filepath.Join(dir, ".bash_history")
	content := "ls\ncd /tmp\ngit status\npwd\necho hello\n"
	if err := os.WriteFile(bashHist, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := &Indexer{
		historyPath: bashHist,
		graph:       hnsw.NewGraph[string](),
		commands:    make(map[string]string),
	}

	cmds := idx.RecentCommands(3)
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[0] != "git status" {
		t.Errorf("expected 'git status', got %q", cmds[0])
	}
	if cmds[2] != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", cmds[2])
	}
}

func TestRecentCommandsMissingFile(t *testing.T) {
	idx := &Indexer{
		historyPath: "/nonexistent/history",
		graph:       hnsw.NewGraph[string](),
		commands:    make(map[string]string),
	}
	cmds := idx.RecentCommands(5)
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for missing file, got %d", len(cmds))
	}
}

func TestNewIndexerNilEmbedder(t *testing.T) {
	idx := NewIndexer(nil, 3000, time.Hour)
	// IndexHistory should be a no-op with nil embedder
	if err := idx.IndexHistory(); err != nil {
		t.Fatalf("unexpected error with nil embedder: %v", err)
	}
}

func TestSearchRelevantNilEmbedder(t *testing.T) {
	idx := NewIndexer(nil, 3000, time.Hour)
	cmds, err := idx.SearchRelevant("test", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmds != nil {
		t.Errorf("expected nil results with nil embedder, got %v", cmds)
	}
}

func TestNewIndexerUsesHISTFILE(t *testing.T) {
	dir := t.TempDir()
	histFile := filepath.Join(dir, "custom_history")
	if err := os.WriteFile(histFile, []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTFILE", histFile)
	idx := NewIndexer(nil, 3000, time.Hour)
	if idx.historyPath != histFile {
		t.Errorf("expected historyPath %s, got %s", histFile, idx.historyPath)
	}
}

func TestHNSWSearchIntegration(t *testing.T) {
	g := hnsw.NewGraph[string]()
	g.Add(
		hnsw.MakeNode("a", []float32{1, 0, 0}),
		hnsw.MakeNode("b", []float32{0.9, 0.1, 0}),
		hnsw.MakeNode("c", []float32{0, 1, 0}),
		hnsw.MakeNode("d", []float32{0, 0, 1}),
	)

	if g.Len() != 4 {
		t.Fatalf("expected 4 nodes, got %d", g.Len())
	}

	// Query near "a" — should return "a" first, then "b"
	results := g.Search([]float32{1, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Key != "a" {
		t.Errorf("expected nearest key 'a', got %q", results[0].Key)
	}
	if results[1].Key != "b" {
		t.Errorf("expected second nearest key 'b', got %q", results[1].Key)
	}

	// Verify Lookup works
	vec, ok := g.Lookup("a")
	if !ok {
		t.Fatal("expected Lookup('a') to succeed")
	}
	if len(vec) != 3 || vec[0] != 1 {
		t.Errorf("unexpected vector for 'a': %v", vec)
	}

	_, ok = g.Lookup("missing")
	if ok {
		t.Error("expected Lookup('missing') to return false")
	}

	// Verify command map integration
	commands := map[string]string{
		"a": "ls -la",
		"b": "ls",
		"c": "git status",
		"d": "docker ps",
	}
	for _, n := range results {
		if commands[n.Key] == "" {
			t.Errorf("missing command for key %q", n.Key)
		}
	}
}

func TestHashCommandDeterministic(t *testing.T) {
	h1 := hashCommand("git status")
	h2 := hashCommand("git status")
	h3 := hashCommand("git log")

	if h1 != h2 {
		t.Error("same command should produce same hash")
	}
	if h1 == h3 {
		t.Error("different commands should produce different hashes")
	}
}
