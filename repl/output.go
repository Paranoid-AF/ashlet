package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	ashlet "github.com/Paranoid-AF/ashlet"
	"github.com/Paranoid-AF/ashlet/generate"
	"golang.org/x/term"
)

// termWriter wraps a file and converts \n to \r\n when the file is a terminal
// (needed because raw mode disables the kernel's NL→CRNL translation).
// When the file is redirected, \n passes through unchanged.
func termWriter(f *os.File) io.Writer {
	if term.IsTerminal(int(f.Fd())) {
		return &crlfWriter{w: f}
	}
	return f
}

type crlfWriter struct {
	w io.Writer
}

func (c *crlfWriter) Write(p []byte) (int, error) {
	replaced := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
	_, err := c.w.Write(replaced)
	return len(p), err // report original length to caller
}

// writeEntry writes a single TOML-formatted entry to w.
func writeEntry(w io.Writer, input string, cursorPos int, cwd string, result *generate.CompleteResult) {
	fmt.Fprintf(w, "# %s\n\n", strings.Repeat("═", 60))

	fmt.Fprintln(w, "[request]")
	fmt.Fprintf(w, "timestamp = %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(w, "input = %s\n", tomlQuote(input))
	fmt.Fprintf(w, "cursor_pos = %d\n", cursorPos)
	fmt.Fprintf(w, "cwd = %s\n", tomlQuote(cwd))
	fmt.Fprintln(w)

	writeContext(w, result)
	writeResponse(w, result.Response)
}

func writeContext(w io.Writer, result *generate.CompleteResult) {
	hasContext := false

	if result.DirContext != nil {
		dc := result.DirContext
		if dc.CwdListing != "" || dc.PackageManager != "" {
			hasContext = true
		}
	}
	if result.Info != nil {
		if len(result.Info.RecentCommands) > 0 || len(result.Info.RelevantCommands) > 0 {
			hasContext = true
		}
	}

	if !hasContext {
		return
	}

	fmt.Fprintln(w, "[context]")

	if dc := result.DirContext; dc != nil {
		if dc.CwdListing != "" {
			fmt.Fprintf(w, "files = %s\n", tomlQuote(dc.CwdListing))
		}
		if dc.PackageManager != "" {
			fmt.Fprintf(w, "package_manager = %s\n", tomlQuote(dc.PackageManager))
		}
		if dc.GitRootListing != "" {
			fmt.Fprintf(w, "project_files = %s\n", tomlQuote(dc.GitRootListing))
		}
		if dc.GitStagedFiles != "" {
			fmt.Fprintf(w, "staged = %s\n", tomlQuote(dc.GitStagedFiles))
		}
		for name, content := range dc.CwdManifests {
			fmt.Fprintf(w, "%s = %s\n", tomlBareKey(name), tomlQuote(content))
		}
		for name, content := range dc.GitManifests {
			fmt.Fprintf(w, "%s = %s\n", tomlBareKey(name), tomlQuote(content))
		}
	}

	if info := result.Info; info != nil {
		if len(info.RecentCommands) > 0 {
			fmt.Fprintf(w, "recent_commands = %s\n", tomlQuote(strings.Join(info.RecentCommands, " | ")))
		}
		if len(info.RelevantCommands) > 0 {
			fmt.Fprintf(w, "relevant_commands = %s\n", tomlQuote(strings.Join(info.RelevantCommands, " | ")))
		}
	}

	fmt.Fprintln(w)
}

func writeResponse(w io.Writer, resp *ashlet.Response) {
	if resp.Error != nil {
		fmt.Fprintln(w, "[error]")
		fmt.Fprintf(w, "code = %s\n", tomlQuote(resp.Error.Code))
		fmt.Fprintf(w, "message = %s\n", tomlQuote(resp.Error.Message))
		fmt.Fprintln(w)
		return
	}

	for _, c := range resp.Candidates {
		fmt.Fprintln(w, "[[candidates]]")
		fmt.Fprintf(w, "completion = %s\n", tomlQuote(c.Completion))
		fmt.Fprintf(w, "confidence = %.2f\n", c.Confidence)
		if c.CursorPos != nil {
			fmt.Fprintf(w, "cursor_pos = %d\n", *c.CursorPos)
		}
		fmt.Fprintln(w)
	}
}

// tomlBareKey converts a key to a valid TOML bare key, quoting if needed.
func tomlBareKey(key string) string {
	bare := strings.ReplaceAll(key, " ", "_")
	for _, c := range bare {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return fmt.Sprintf("%q", key)
		}
	}
	return bare
}

// tomlQuote returns a TOML basic-string quoted value.
func tomlQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return "\"" + s + "\""
}
