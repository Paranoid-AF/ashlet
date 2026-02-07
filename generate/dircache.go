package generate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jellydator/ttlcache/v3"
)

// DirContext holds gathered context for one directory.
type DirContext struct {
	CwdPath        string
	CwdListing     string            // ls -a output (space-separated)
	CwdManifests   map[string]string // filename label -> extracted content
	PackageManager string            // detected from lockfile (pnpm, yarn, bun, npm, cargo)
	GitRoot        string
	GitRootListing string
	GitStagedFiles string
	GitLog         string
	GitManifests   map[string]string // manifest files at git root (if different from cwd)
}

const (
	dirCacheTTL      = 1 * time.Hour
	gatherTimeout    = 5 * time.Second
	manifestMaxBytes = 512
	fieldMaxBytes    = 512
)

// DirCache is a TTL cache of DirContext entries keyed by absolute path.
type DirCache struct {
	cache *ttlcache.Cache[string, *DirContext]
}

// NewDirCache creates a new DirCache with TTL-based expiration.
func NewDirCache() *DirCache {
	c := ttlcache.New[string, *DirContext](
		ttlcache.WithTTL[string, *DirContext](dirCacheTTL),
		ttlcache.WithDisableTouchOnHit[string, *DirContext](),
	)
	go c.Start()
	return &DirCache{cache: c}
}

// Close stops the cache expiration loop.
func (dc *DirCache) Close() {
	dc.cache.Stop()
}

// Get returns the cached DirContext for the given path, or nil if not cached/expired.
func (dc *DirCache) Get(absPath string) *DirContext {
	item := dc.cache.Get(absPath)
	if item == nil {
		return nil
	}
	return item.Value()
}

// Gather collects directory context for the given path and caches it.
func (dc *DirCache) Gather(ctx context.Context, cwd string) {
	ctx, cancel := context.WithTimeout(ctx, gatherTimeout)
	defer cancel()

	entry := &DirContext{
		CwdPath:      cwd,
		CwdManifests: make(map[string]string),
		GitManifests: make(map[string]string),
	}

	type result struct {
		key string
		val string
	}
	ch := make(chan result, 10)

	var wg sync.WaitGroup

	// ls -a (cwd)
	wg.Add(1)
	go func() {
		defer wg.Done()
		out := runCmd(ctx, cwd, "ls", "-a")
		listing := strings.Join(strings.Fields(out), " ")
		ch <- result{"cwd_listing", truncate(listing, fieldMaxBytes)}
	}()

	// git root
	wg.Add(1)
	go func() {
		defer wg.Done()
		out := strings.TrimSpace(runCmd(ctx, cwd, "git", "rev-parse", "--show-toplevel"))
		ch <- result{"git_root", out}
	}()

	// git staged (single-line, space-separated)
	wg.Add(1)
	go func() {
		defer wg.Done()
		out := strings.TrimSpace(runCmd(ctx, cwd, "git", "diff", "--cached", "--name-only"))
		ch <- result{"git_staged", toSingleLine(out, fieldMaxBytes)}
	}()

	// git log (single-line, semicolon-separated)
	wg.Add(1)
	go func() {
		defer wg.Done()
		out := strings.TrimSpace(runCmd(ctx, cwd, "git", "log", "--oneline", "-5"))
		ch <- result{"git_log", toSingleLine(out, fieldMaxBytes)}
	}()

	// Collect parallel results
	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		switch r.key {
		case "cwd_listing":
			entry.CwdListing = r.val
		case "git_root":
			entry.GitRoot = r.val
		case "git_staged":
			entry.GitStagedFiles = r.val
		case "git_log":
			entry.GitLog = r.val
		}
	}

	// After git root is known, gather git-root listing and manifests
	if entry.GitRoot != "" && entry.GitRoot != cwd {
		out := runCmd(ctx, entry.GitRoot, "ls", "-a")
		entry.GitRootListing = truncate(strings.Join(strings.Fields(out), " "), fieldMaxBytes)
		gatherManifests(ctx, entry.GitRoot, entry.GitManifests)
	}

	// Gather cwd manifests
	gatherManifests(ctx, cwd, entry.CwdManifests)

	// Detect package manager
	entry.PackageManager = detectPackageManager(cwd, entry.GitRoot)

	dc.cache.Set(cwd, entry, ttlcache.DefaultTTL)

	slog.Debug("gathered directory context", "path", cwd)
}

// runCmd runs a command and returns its stdout, or empty string on error.
func runCmd(ctx context.Context, dir string, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// manifestFiles lists the manifest filenames to look for.
var manifestFiles = []string{
	"package.json",
	"Makefile",
	"Cargo.toml",
	"pyproject.toml",
	"go.mod",
	"CMakeLists.txt",
}

func gatherManifests(ctx context.Context, dir string, out map[string]string) {
	for _, name := range manifestFiles {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var extracted string
		switch name {
		case "package.json":
			extracted = extractPackageJsonScripts(string(data))
		case "Makefile":
			extracted = extractMakefileTargets(string(data))
		case "Cargo.toml":
			extracted = extractCargoInfo(string(data))
		case "go.mod":
			extracted = extractGoModInfo(string(data))
		case "pyproject.toml":
			extracted = extractPyprojectInfo(string(data))
		case "CMakeLists.txt":
			extracted = extractCMakeInfo(string(data))
		}

		if extracted != "" {
			label := name
			if name == "package.json" {
				label = "package.json scripts"
			} else if name == "Makefile" {
				label = "Makefile targets"
			}
			out[label] = extracted
		}
	}
}

// extractPackageJsonScripts extracts the "scripts" object from package.json.
func extractPackageJsonScripts(content string) string {
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return ""
	}
	scripts, ok := pkg["scripts"]
	if !ok {
		return ""
	}
	var s map[string]string
	if err := json.Unmarshal(scripts, &s); err != nil {
		return ""
	}
	// Format as "key: value" pairs
	parts := make([]string, 0, len(s))
	for k, v := range s {
		parts = append(parts, k+": "+v)
	}
	return truncate(strings.Join(parts, ", "), manifestMaxBytes)
}

// extractMakefileTargets extracts target names from a Makefile.
func extractMakefileTargets(content string) string {
	var targets []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		// Match lines like "target:" or "target: deps"
		// Skip lines starting with tab (recipe lines) or # (comments)
		if len(line) == 0 || line[0] == '\t' || line[0] == '#' || line[0] == '.' {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		// Skip assignment operators (:=)
		if idx+1 < len(line) && line[idx+1] == '=' {
			continue
		}
		target := strings.TrimSpace(line[:idx])
		// Skip targets with variables or special chars
		if strings.ContainsAny(target, "$%") {
			continue
		}
		if !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
	}
	return truncate(strings.Join(targets, ", "), manifestMaxBytes)
}

type cargoToml struct {
	Package struct {
		Name string `toml:"name"`
	} `toml:"package"`
	Bin []struct {
		Name string `toml:"name"`
	} `toml:"bin"`
}

// extractCargoInfo extracts name and [[bin]] targets from Cargo.toml.
func extractCargoInfo(content string) string {
	var cargo cargoToml
	if _, err := toml.Decode(content, &cargo); err != nil {
		return ""
	}
	var parts []string
	if cargo.Package.Name != "" {
		parts = append(parts, fmt.Sprintf(`name = "%s"`, cargo.Package.Name))
	}
	for _, bin := range cargo.Bin {
		if bin.Name != "" {
			parts = append(parts, fmt.Sprintf(`name = "%s"`, bin.Name))
		}
	}
	return truncate(strings.Join(parts, ", "), manifestMaxBytes)
}

// lockfileMap maps lockfile names to package manager names.
// Ordered by priority (more specific lockfiles first).
var lockfileMap = []struct {
	file    string
	manager string
}{
	{"pnpm-lock.yaml", "pnpm"},
	{"yarn.lock", "yarn"},
	{"bun.lockb", "bun"},
	{"package-lock.json", "npm"},
	{"Cargo.lock", "cargo"},
}

// detectPackageManager detects the package manager from lockfile presence.
// Checks cwd first, then git root.
func detectPackageManager(cwd, gitRoot string) string {
	for _, dirs := range []string{cwd, gitRoot} {
		if dirs == "" {
			continue
		}
		for _, lf := range lockfileMap {
			if _, err := os.Stat(filepath.Join(dirs, lf.file)); err == nil {
				return lf.manager
			}
		}
	}
	return ""
}

// toSingleLine converts a multi-line string to a single line (space-separated)
// and caps the total length.
func toSingleLine(s string, maxBytes int) string {
	if s == "" {
		return ""
	}
	joined := strings.Join(strings.Fields(s), " ")
	return truncate(joined, maxBytes)
}

// extractGoModInfo extracts module path and Go version from go.mod.
func extractGoModInfo(content string) string {
	var parts []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			parts = append(parts, line)
		} else if strings.HasPrefix(line, "go ") && !strings.HasPrefix(line, "go.") {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, ", ")
}

type pyprojectToml struct {
	Project struct {
		Name string `toml:"name"`
	} `toml:"project"`
}

// extractPyprojectInfo extracts project name from pyproject.toml.
func extractPyprojectInfo(content string) string {
	var pyproject pyprojectToml
	if _, err := toml.Decode(content, &pyproject); err != nil {
		return ""
	}
	if pyproject.Project.Name == "" {
		return ""
	}
	return fmt.Sprintf(`name = "%s"`, pyproject.Project.Name)
}

// extractCMakeInfo extracts project name from CMakeLists.txt.
func extractCMakeInfo(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "project(") || strings.HasPrefix(lower, "project (") {
			return truncate(line, manifestMaxBytes)
		}
	}
	return ""
}

// truncate truncates s to maxBytes, appending "..." if truncated.
func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "..."
}
