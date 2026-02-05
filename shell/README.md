# ashlet — Shell Integration

Shell client for ashlet. Provides Zsh and Bash integration for AI-powered auto-completion.

## Setup

### Zsh
Add to your `.zshrc`:
```zsh
source /path/to/ashlet/shell/ashlet.zsh
```

### Bash
Add to your `.bashrc`:
```bash
source /path/to/ashlet/shell/ashlet.bash
```

## Usage

Press `Ctrl+Space` to trigger AI completion at the cursor position.

## Dependencies

- `socat` — for Unix domain socket communication
- `jq` (optional) — for JSON parsing (falls back to grep-based extraction)

## Configuration

| Variable | Description | Default |
|---|---|---|
| `ASHLET_SOCKET` | Override socket path | `$XDG_RUNTIME_DIR/ashlet.sock` or `/tmp/ashlet-$UID.sock` |
