# ashlet — Shell Integration

Shell client for ashlet. Provides Zsh integration for AI-powered auto-completion.

## Setup

Add to your `.zshrc`:
```zsh
source /path/to/ashlet/shell/ashlet.zsh
```

## Usage

A single candidate line is always visible below your prompt showing the top suggestion. Candidates update automatically as you type. If a candidate includes a cursor position, `▮` marks where your cursor will land.

- **Tab** — Apply the displayed candidate (replaces your input). Falls through to default completion if no candidates.
- **Shift+Tab** — Default Zsh completion (`expand-or-complete`)
- **Shift+Left/Right** — Navigate between candidates
- **ESC** — Dismiss candidate display until next command
- **Enter** — Executes the command normally (not intercepted by ashlet)
- **Continue typing** — Fetches new suggestions automatically

## Dependencies

- `socat` — for Unix domain socket communication (required)
- `jq` — for JSON parsing (required)

## Configuration

| Variable | Description | Default |
|---|---|---|
| `ASHLET_SOCKET` | Override socket path | `$XDG_RUNTIME_DIR/ashlet.sock` or `/tmp/ashlet-$UID.sock` |
| `ASHLET_MAX_CANDIDATES` | Maximum number of candidates to request | `4` |
