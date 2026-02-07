# ashlet — AI-powered shell auto-completion

## Project Overview

ashlet is a shell auto-completion system powered by AI. It uses OpenAI-compatible APIs (OpenRouter by default, or any provider like Ollama) for inference, with no local model files required.

## Architecture

Flat package layout:

1. **shell/** — Shell client (Zsh integration). Captures input context, sends requests to daemon via Unix domain socket, applies completions to the input buffer.
2. **Root package (`ashlet`)** — Shared IPC types (`ashlet.go`) and configuration (`config.go`).
3. **index/** — History indexing and embedding via API.
4. **generate/** — Completion orchestration, context gathering, and inference via API.
5. **serve/** — Daemon entry point and Unix socket server (`ashletd`).

Dependency graph (no cycles): `root (ashlet) ← index ← generate ← serve (main)`

## IPC

- **Mechanism**: Unix domain sockets (file-system based, bidirectional)
- **Protocol**: JSON over socket (see `ashlet.go`)
- **Socket path**: `$XDG_RUNTIME_DIR/ashlet.sock` or `/tmp/ashlet-$UID.sock`
- **Response format**: `{"candidates": [...], "error": {"code": "...", "message": "..."}}`
- **Error codes**: `not_configured` — API key missing, `api_error` — API request failed

## Configuration

Config file: `~/.config/ashlet/config.json` (created on-demand via `ashlet` command)
Prompt file: `~/.config/ashlet/prompt.md` (created on-demand via `ashlet` command)

### Config Schema

```json
{
  "version": 1,
  "generation": {
    "base_url": "https://openrouter.ai/api/v1",
    "api_key": "",
    "api_type": "responses",
    "model": "inception/mercury-coder",
    "max_tokens": 120,
    "temperature": 0.3
  },
  "embedding": {
    "base_url": "",
    "api_key": "",
    "model": "openai/text-embedding-3-small",
    "dimensions": 1536,
    "ttl_minutes": 60,
    "max_history_commands": 3000
  },
  "telemetry": {
    "openrouter": true
  }
}
```

### API Key Resolution

- **Generation**: `$ASHLET_GENERATION_API_KEY` env var > `generation.api_key` in config
- **Embedding**: `$ASHLET_EMBEDDING_API_KEY` env var > `embedding.api_key` in config
- Embedding is disabled when `base_url` or `api_key` is empty (graceful degradation to recency-only history)

### API Types

- `"responses"` (default): OpenAI Responses API format (`POST /responses`)
- `"chat_completions"`: Chat Completions format (`POST /chat/completions`) for providers like Ollama

### Telemetry

When `telemetry.openrouter` is true (default), attribution headers are sent:
- `X-Title: Ashlet - auto complete your shell commands`
- `HTTP-Referer: https://github.com/Paranoid-AF/ashlet`

## Build Commands

```bash
# Bootstrap (download Go deps)
make bootstrap

# Top-level
make build
make test
make lint
make clean
```

## Test Commands

```bash
make test                # All tests (Go + shell)
go test ./...            # Go tests only
cd shell && bats tests/  # Shell tests (requires bats-core)
```

## Go Module

Single Go module at project root:

- **Module**: `github.com/Paranoid-AF/ashlet`

### Packages

- `ashlet.go` — shared IPC request/response types
- `config.go` — configuration types and path resolution
- `serve/` — daemon entry point and Unix socket server
- `generate/` — completion orchestration, context gathering, inference via API
- `index/` — history indexing, embedding via API

## Development

Two `_run.sh` scripts launch the daemon and shell client separately. Open two terminals to test end-to-end:

**Terminal 1 — daemon** (`_run.sh` at project root):

```bash
./_run.sh            # build ashletd and start it
./_run.sh --verbose  # same, with request/response logging
```

Runs `make build` then `exec ./ashletd`. Stays in the foreground so you see logs. Use `--verbose` to log every request/response.

**Terminal 2 — shell client** (`shell/_run.sh`):

```bash
./shell/_run.sh
```

Checks for dependencies (`zsh`, `socat`, `jq`), then launches an interactive zsh session with ashlet pre-loaded via a temporary `ZDOTDIR`. Type commands to see completions.

## Design Constraints

- Inference via OpenAI-compatible APIs (no local model files)
- File-based IPC only (Unix domain sockets, no TCP)
- Shell integration must handle cursor position manipulation correctly
- Shell integration is Zsh-only (requires Zsh 5.3+)
- Config/prompt files created on-demand via `ashlet` command only
- Embeddings stored in-memory with TTL (no disk persistence)
- `jq` is a required dependency for shell integrations (no grep fallback)
