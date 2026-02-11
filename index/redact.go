package index

import (
	"bytes"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// safeVars are environment variables that are non-sensitive and useful for LLM context.
var safeVars = map[string]bool{
	"HOME": true, "USER": true, "PWD": true, "OLDPWD": true,
	"SHELL": true, "PATH": true, "LANG": true, "TERM": true,
	"EDITOR": true, "PAGER": true, "HOSTNAME": true, "LOGNAME": true,
	"TMPDIR": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true,
	"XDG_RUNTIME_DIR": true, "DISPLAY": true, "WAYLAND_DISPLAY": true,
	"HISTFILE": true, "HISTSIZE": true, "SHLVL": true,
	"COLUMNS": true, "LINES": true, "LC_ALL": true, "LC_CTYPE": true,
}

// specialParams are shell special parameters that should not be redacted.
var specialParams = map[string]bool{
	"?": true, "!": true, "#": true, "@": true, "*": true,
	"-": true, "$": true, "_": true,
	"0": true, "1": true, "2": true, "3": true, "4": true,
	"5": true, "6": true, "7": true, "8": true, "9": true,
}

// RedactCommand replaces sensitive environment variable references and
// assignment values in a shell command string. Safe variables (PATH, HOME, etc.)
// and special shell parameters ($?, $!, etc.) are preserved.
func RedactCommand(cmd string) string {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash), syntax.KeepComments(true))
	prog, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return regexRedact(cmd)
	}

	syntax.Walk(prog, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.ParamExp:
			if n.Param != nil && !safeVars[n.Param.Value] && !specialParams[n.Param.Value] {
				n.Param.Value = "REDACTED"
			}
		case *syntax.Assign:
			if n.Name != nil && !safeVars[n.Name.Value] && n.Value != nil {
				n.Value.Parts = []syntax.WordPart{&syntax.Lit{Value: "***"}}
			}
		}
		return true
	})

	var buf bytes.Buffer
	printer := syntax.NewPrinter(syntax.Indent(0))
	if err := printer.Print(&buf, prog); err != nil {
		return regexRedact(cmd)
	}
	return strings.TrimRight(buf.String(), "\n")
}

// RedactCommands applies RedactCommand to each element.
func RedactCommands(cmds []string) []string {
	out := make([]string, len(cmds))
	for i, cmd := range cmds {
		out[i] = RedactCommand(cmd)
	}
	return out
}

// FilterQuoteContent strips text inside quotes from a command string.
// Double-quoted content becomes "" and single-quoted content becomes ”.
// Handles escaped quotes inside quoted strings.
func FilterQuoteContent(cmd string) string {
	var buf strings.Builder
	buf.Grow(len(cmd))
	i := 0
	for i < len(cmd) {
		ch := cmd[i]
		if ch == '"' || ch == '\'' {
			quote := ch
			buf.WriteByte(quote)
			i++
			for i < len(cmd) {
				if cmd[i] == '\\' && i+1 < len(cmd) {
					i += 2
					continue
				}
				if cmd[i] == quote {
					break
				}
				i++
			}
			if i < len(cmd) {
				buf.WriteByte(quote)
				i++
			}
		} else {
			buf.WriteByte(ch)
			i++
		}
	}
	return buf.String()
}

// FilterQuoteContentSlice applies FilterQuoteContent to each element and deduplicates.
func FilterQuoteContentSlice(cmds []string) []string {
	seen := make(map[string]bool, len(cmds))
	out := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		filtered := FilterQuoteContent(cmd)
		if !seen[filtered] {
			seen[filtered] = true
			out = append(out, filtered)
		}
	}
	return out
}

var (
	reBraceVar  = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	reSimpleVar = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	reAssign    = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)=(\S+)`)
)

// regexRedact is a fallback for commands that fail AST parsing.
func regexRedact(cmd string) string {
	// ${VAR} → ${REDACTED}
	cmd = reBraceVar.ReplaceAllStringFunc(cmd, func(m string) string {
		name := reBraceVar.FindStringSubmatch(m)[1]
		if safeVars[name] || specialParams[name] {
			return m
		}
		return "${REDACTED}"
	})

	// $VAR → $REDACTED
	cmd = reSimpleVar.ReplaceAllStringFunc(cmd, func(m string) string {
		name := reSimpleVar.FindStringSubmatch(m)[1]
		if name == "REDACTED" { // already redacted by brace pass
			return m
		}
		if safeVars[name] || specialParams[name] {
			return m
		}
		return "$REDACTED"
	})

	// VAR=value → VAR=***
	cmd = reAssign.ReplaceAllStringFunc(cmd, func(m string) string {
		parts := reAssign.FindStringSubmatch(m)
		name := parts[1]
		if safeVars[name] {
			return m
		}
		return name + "=***"
	})

	return cmd
}
