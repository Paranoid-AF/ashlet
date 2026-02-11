// Command ashlet-repl is an interactive test REPL for ashlet completions.
// It uses raw terminal input to track cursor position natively and writes
// structured TOML results to stdout.
//
// Usage:
//
//	./ashlet-repl             # interactive, TOML on screen
//	./ashlet-repl > log.toml  # prompt on screen, TOML to file
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	ashlet "github.com/Paranoid-AF/ashlet"
	"github.com/Paranoid-AF/ashlet/generate"
)

const prompt = "> "

func main() {
	editor, err := NewEditor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer editor.Close()

	tty := editor.Tty()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(tty, "error: cannot determine cwd: %v\r\n", err)
		os.Exit(1)
	}

	// Embedding cache in project root .cache/
	cacheDir := filepath.Join(cwd, ".cache")
	os.MkdirAll(cacheDir, 0755)
	cachePath := filepath.Join(cacheDir, "embeddings.json")

	fmt.Fprintf(tty, "\033[2J\033[H") // clear screen
	fmt.Fprintf(tty, "ashlet repl\r\n")
	fmt.Fprintf(tty, "cwd: %s\r\n", cwd)
	fmt.Fprintf(tty, "\r\ncommands:\r\n")
	fmt.Fprintf(tty, "  :cwd <path>  set working directory\r\n")
	fmt.Fprintf(tty, "  :quit        exit\r\n\r\n")

	engine := generate.NewEngine()
	defer engine.Close()

	// Load previous embedding cache before the refresh loop gets far.
	if err := engine.LoadIndexCache(cachePath); err != nil {
		slog.Debug("no embedding cache loaded", "error", err)
	}
	defer func() {
		if err := engine.SaveIndexCache(cachePath); err != nil {
			slog.Warn("failed to save embedding cache", "error", err)
		}
	}()

	engine.WarmContext(context.Background(), cwd)

	// stdout writer: converts \n → \r\n when stdout is a terminal (raw mode),
	// passes \n through unchanged when redirected to a file.
	out := termWriter(os.Stdout)

	reqID := 0

	for {
		text, cursorPos, err := editor.ReadLine(prompt)
		if err == io.EOF || err == ErrInterrupt {
			break
		}
		if err != nil {
			fmt.Fprintf(tty, "read error: %v\r\n", err)
			break
		}

		if text == "" {
			continue
		}

		if text == ":quit" || text == ":q" {
			break
		}

		if strings.HasPrefix(text, ":cwd ") {
			newCwd := strings.TrimSpace(strings.TrimPrefix(text, ":cwd "))
			info, statErr := os.Stat(newCwd)
			if statErr != nil || !info.IsDir() {
				fmt.Fprintf(tty, "error: not a directory: %s\r\n", newCwd)
				continue
			}
			cwd = newCwd
			engine.WarmContext(context.Background(), cwd)
			fmt.Fprintf(tty, "cwd: %s\r\n\r\n", cwd)
			continue
		}

		reqID++
		req := &ashlet.Request{
			RequestID: reqID,
			Input:     text,
			CursorPos: cursorPos,
			Cwd:       cwd,
			SessionID: "repl",
		}

		result := engine.CompleteVerbose(context.Background(), req)
		resp := result.Response

		// Show brief summary on tty.
		if resp.Error != nil {
			fmt.Fprintf(tty, "error [%s]: %s\r\n", resp.Error.Code, resp.Error.Message)
		} else if len(resp.Candidates) == 0 {
			fmt.Fprintf(tty, "(no candidates)\r\n")
		} else {
			for i, c := range resp.Candidates {
				cursor := "end"
				if c.CursorPos != nil {
					cursor = fmt.Sprintf("%d", *c.CursorPos)
				}
				fmt.Fprintf(tty, "  %d. [%.2f] %s (cursor: %s)\r\n", i+1, c.Confidence, c.Completion, cursor)
			}
		}
		fmt.Fprintf(tty, "\r\n")

		// TOML output to stdout (crlfWriter handles raw mode).
		writeEntry(out, text, cursorPos, cwd, result)
	}
}
