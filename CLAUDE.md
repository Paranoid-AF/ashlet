# ashlet — AI-powered shell auto-completion

## Project Overview

ashlet is a shell auto-completion system powered by local CPU-based AI. It runs entirely on-device using llama.cpp with GGUF models, optimized for Apple Silicon Metal.

## Architecture

Three-component monorepo:

1. **shell/** — Shell client (Zsh/Bash integration). Captures input context, sends requests to daemon via Unix domain socket, applies completions to the input buffer.
2. **daemon/** — Go daemon (`ashletd`). Listens on a Unix domain socket, gathers context (cwd, git state, command history), orchestrates model inference, returns completions.
3. **model/** — Model layer. Wraps llama.cpp for embedding generation and text inference. Manages GGUF model files.

## IPC

- **Mechanism**: Unix domain sockets (file-system based, bidirectional)
- **Protocol**: JSON over socket (see `daemon/pkg/protocol/protocol.go`)
- **Socket path**: `$XDG_RUNTIME_DIR/ashlet.sock` or `/tmp/ashlet-$UID.sock`

## Build Commands

```bash
# Top-level (builds everything)
make build
make test
make lint
make clean

# Per-component
cd daemon && make build
cd model && make build
```

## Test Commands

```bash
make test              # All tests
cd daemon && make test # Go tests
cd shell && bats tests/ # Shell tests (requires bats-core)
```

## Go Module

- **Module path**: `github.com/Paranoid-AF/ashlet`
- **Go source**: `daemon/` directory
- **Key packages**:
  - `cmd/ashletd` — daemon entry point
  - `internal/ipc` — Unix socket server
  - `internal/history` — shell history indexer
  - `internal/context` — context gathering (git, cwd, recent commands)
  - `internal/completion` — completion orchestration
  - `pkg/protocol` — shared request/response types

## Design Constraints

- All inference runs locally on CPU/Metal — no network calls to external AI services
- File-based IPC only (Unix domain sockets, no TCP)
- Shell integration must handle cursor position manipulation correctly
- Must support both Zsh and Bash
- Model weights are not checked into git (downloaded via setup script)
