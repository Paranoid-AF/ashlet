// Package context gathers contextual information for completion requests.
// This includes git state, recent commands, and working directory info.
package context

import (
	"os/exec"
	"strings"

	"github.com/Paranoid-AF/ashlet/internal/history"
	"github.com/Paranoid-AF/ashlet/pkg/protocol"
)

// Info holds gathered context for a completion request.
type Info struct {
	RecentCommands []string
	GitBranch      string
	GitDiff        string
	DirContents    []string
}

// Gatherer collects context for completion requests.
type Gatherer struct {
	historyIndexer *history.Indexer
}

// NewGatherer creates a new context gatherer.
func NewGatherer() *Gatherer {
	return &Gatherer{
		historyIndexer: history.NewIndexer(),
	}
}

// Gather collects context based on the completion request.
func (g *Gatherer) Gather(req *protocol.Request) *Info {
	info := &Info{
		RecentCommands: g.historyIndexer.RecentCommands(20),
	}

	if req.Cwd != "" {
		info.GitBranch = gitBranch(req.Cwd)
		info.GitDiff = gitDiffStat(req.Cwd)
	}

	return info
}

func gitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitDiffStat(dir string) string {
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
