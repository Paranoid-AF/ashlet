package generate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

func TestDirCacheGetMiss(t *testing.T) {
	dc := NewDirCache()
	defer dc.Close()

	if got := dc.Get("/nonexistent/path"); got != nil {
		t.Errorf("expected nil for cache miss, got %+v", got)
	}
}

func TestDirCacheGetHit(t *testing.T) {
	dc := NewDirCache()
	defer dc.Close()

	dc.cache.Set("/test", &DirContext{
		CwdPath:    "/test",
		CwdListing: "a b c",
	}, ttlcache.DefaultTTL)

	got := dc.Get("/test")
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if got.CwdListing != "a b c" {
		t.Errorf("expected listing %q, got %q", "a b c", got.CwdListing)
	}
}

func TestDirCacheGetExpired(t *testing.T) {
	// Create a cache with a very short TTL
	c := ttlcache.New[string, *DirContext](
		ttlcache.WithTTL[string, *DirContext](time.Millisecond),
		ttlcache.WithDisableTouchOnHit[string, *DirContext](),
	)
	go c.Start()
	dc := &DirCache{cache: c}
	defer dc.Close()

	dc.cache.Set("/test", &DirContext{CwdPath: "/test"}, ttlcache.DefaultTTL)
	time.Sleep(10 * time.Millisecond)

	if got := dc.Get("/test"); got != nil {
		t.Errorf("expected nil for expired entry, got %+v", got)
	}
}

func TestDirCacheGatherOverwrite(t *testing.T) {
	dc := NewDirCache()
	defer dc.Close()

	dir := t.TempDir()

	dc.Gather(context.Background(), dir)
	first := dc.Get(dir)
	if first == nil {
		t.Fatal("expected entry after first gather")
	}

	dc.Gather(context.Background(), dir)
	second := dc.Get(dir)
	if second == nil {
		t.Fatal("expected entry after second gather")
	}
}

func TestDirCacheGatherPopulatesListing(t *testing.T) {
	dc := NewDirCache()
	defer dc.Close()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0644)

	dc.Gather(context.Background(), dir)
	got := dc.Get(dir)
	if got == nil {
		t.Fatal("expected entry")
	}
	if !strings.Contains(got.CwdListing, "hello.txt") {
		t.Errorf("expected listing to contain hello.txt, got %q", got.CwdListing)
	}
}

func TestExtractPackageJSONScripts(t *testing.T) {
	content := `{
		"name": "myapp",
		"scripts": {
			"build": "tsc",
			"test": "jest",
			"start": "node ."
		}
	}`
	result := extractPackageJSONScripts(content)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result, "build") || !strings.Contains(result, "test") {
		t.Errorf("expected scripts keys, got %q", result)
	}
}

func TestExtractPackageJSONScriptsNoScripts(t *testing.T) {
	content := `{"name": "myapp", "version": "1.0.0"}`
	result := extractPackageJSONScripts(content)
	if result != "" {
		t.Errorf("expected empty for no scripts, got %q", result)
	}
}

func TestExtractMakefileTargets(t *testing.T) {
	content := `# Makefile
.PHONY: build test

build:
	go build ./...

test: build
	go test ./...

clean:
	rm -rf bin/

VERSION := 1.0
`
	result := extractMakefileTargets(content)
	if !strings.Contains(result, "build") {
		t.Errorf("expected 'build' target, got %q", result)
	}
	if !strings.Contains(result, "test") {
		t.Errorf("expected 'test' target, got %q", result)
	}
	if !strings.Contains(result, "clean") {
		t.Errorf("expected 'clean' target, got %q", result)
	}
	if strings.Contains(result, "VERSION") {
		t.Errorf("should not include assignment as target, got %q", result)
	}
}

func TestExtractJustfileRecipes(t *testing.T) {
	content := `# Justfile
bun := "bun"

# Default target
default: list

# Development
dev:
    @echo "Starting..."
    cd apps && bun run start

build: build-server build-web
    @echo "Done"

# Cleanup
clean:
    rm -rf dist/

list:
    @just --list
`
	result := extractJustfileRecipes(content)
	for _, want := range []string{"default", "dev", "build", "clean", "list"} {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q recipe, got %q", want, result)
		}
	}
	if strings.Contains(result, "bun") {
		t.Errorf("should not include variable assignment as recipe, got %q", result)
	}
}

func TestExtractCargoInfo(t *testing.T) {
	content := `[package]
name = "myapp"
version = "0.1.0"

[[bin]]
name = "mycli"
`
	result := extractCargoInfo(content)
	if !strings.Contains(result, "myapp") {
		t.Errorf("expected package name, got %q", result)
	}
}

func TestToSingleLine(t *testing.T) {
	multi := "file1.go\nfile2.go\nfile3.go"
	got := toSingleLine(multi, 100)
	if got != "file1.go file2.go file3.go" {
		t.Errorf("expected space-separated, got %q", got)
	}

	if toSingleLine("", 100) != "" {
		t.Error("expected empty for empty input")
	}

	long := strings.Repeat("x\n", 500)
	got = toSingleLine(long, 20)
	if len(got) > 23 { // 20 + "..."
		t.Errorf("expected capped length, got %d", len(got))
	}
}

func TestExtractGoModInfo(t *testing.T) {
	content := `module github.com/Paranoid-AF/ashlet

go 1.24.0

require (
	github.com/mattn/go-sqlite3 v1.14.28
)
`
	result := extractGoModInfo(content)
	if !strings.Contains(result, "module github.com/Paranoid-AF/ashlet") {
		t.Errorf("expected module path, got %q", result)
	}
	if !strings.Contains(result, "go 1.24.0") {
		t.Errorf("expected go version, got %q", result)
	}
	if strings.Contains(result, "require") {
		t.Errorf("should not include require block, got %q", result)
	}
}

func TestExtractPyprojectInfo(t *testing.T) {
	content := `[project]
name = "myapp"
version = "0.1.0"
`
	result := extractPyprojectInfo(content)
	if !strings.Contains(result, "myapp") {
		t.Errorf("expected project name, got %q", result)
	}
}

func TestExtractCMakeInfo(t *testing.T) {
	content := `cmake_minimum_required(VERSION 3.10)
project(MyApp VERSION 1.0)
add_executable(myapp main.cpp)
`
	result := extractCMakeInfo(content)
	if !strings.Contains(result, "project(MyApp") {
		t.Errorf("expected project name, got %q", result)
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if got := truncate(short, 100); got != short {
		t.Errorf("expected %q unchanged, got %q", short, got)
	}

	long := strings.Repeat("x", 3000)
	got := truncate(long, 2048)
	if len(got) != 2048+3 { // 2048 + "..."
		t.Errorf("expected length %d, got %d", 2048+3, len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected truncated string to end with ...")
	}
}

func TestDetectPackageManager(t *testing.T) {
	dir := t.TempDir()

	// No lockfile
	if got := detectPackageManager(dir, ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// Create pnpm lockfile
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644)
	if got := detectPackageManager(dir, ""); got != "pnpm" {
		t.Errorf("expected pnpm, got %q", got)
	}
}

func TestGatherManifests(t *testing.T) {
	dir := t.TempDir()

	// Create a package.json with scripts
	pkgJSON := `{"name":"test","scripts":{"build":"go build","test":"go test"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644)

	out := make(map[string]string)
	gatherManifests(dir, out)

	if _, ok := out["package.json scripts"]; !ok {
		t.Error("expected package.json scripts in manifest output")
	}
}
