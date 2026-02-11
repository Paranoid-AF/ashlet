# ashlet

[![GitHub Release](https://img.shields.io/github/v/release/Paranoid-AF/ashlet)](https://github.com/Paranoid-AF/ashlet/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/Paranoid-AF/ashlet/blob/master/LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/Paranoid-AF/ashlet)](https://github.com/Paranoid-AF/ashlet/stargazers)

![Live Demo](https://github.com/Paranoid-AF/ashlet/blob/master/.assets/readme/demo.gif?raw=true)

AI-powered shell auto-completion for Zsh. Suggestions appear as you type, powered by any OpenAI-compatible API.

ashlet runs a lightweight daemon (`ashletd`) that gathers context from your shell — working directory, command history, git status, project manifests — and sends it to an inference API. Candidates are streamed back and displayed inline below your prompt.

```
Zsh Shell <--> ashlet.zsh <--> (Unix socket) <--> ashletd <--> API provider
```

## Table of Contents

- [Quickstart](#quickstart)
- [Requirements](#requirements)
- [Build from Source](#build-from-source)
- [How It Works](#how-it-works)
- [Privacy](#privacy)
- [Keybindings](#keybindings)
- [Troubleshooting](#troubleshooting)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Development](#development)
- [Why Name It `ashlet`?](#why-name-it-ashlet)
- [License](#license)

## Quickstart

```bash
# 1) Install
brew install paranoid-af/tap/ashlet

# 2) Enable in Zsh
echo 'source $(brew --prefix)/share/ashlet/ashlet.zsh' >> ~/.zshrc
exec zsh

# 3) Start the daemon
brew services start ashlet

# 4) Configure an API key (recommended)
ashlet   # creates ~/.config/ashlet/config.json and ~/.config/ashlet/prompt.md

# If you are running ./ashletd manually (not via brew services), you can also:
# export ASHLET_GENERATION_API_KEY="your-openrouter-key"
```

Type a command, wait for a suggestion, then press `Tab` to accept.

Try it:

```bash
git st
```

## Requirements

- **Zsh** 5.3+
- **Go** 1.25+ (to build from source)
- **socat** and **jq** (runtime)

## Build from Source

```bash
# Build the daemon
make build

# Start it
./ashletd

# In your .zshrc
source /path/to/ashlet/shell/ashlet.zsh
```

Set an API key to enable completions:

```bash
export ASHLET_GENERATION_API_KEY="your-openrouter-key"
```

Or run `ashlet` in your shell to launch the configuration TUI, which creates `~/.config/ashlet/config.json`.

## How It Works

```
You type → Zsh hook fires → JSON request over Unix socket → ashletd gathers context →
API call → candidates parsed → displayed below prompt → Tab to accept
```

The daemon gathers rich context for each request:

- **Shell history** — recent commands + semantically relevant ones (via optional embeddings)
- **Directory listing** — files, detected package manager
- **Git info** — repo root, staged files, recent commits, manifests (package.json, Makefile, etc.)
- **Cursor position** — understands partial tokens

## Privacy

ashlet sends context to your configured API provider to generate completions.

- **What gets sent**:
  - **The line you are typing** (and cursor position)
  - **Local context** like directory info and (optionally) git metadata
  - **History context**:
    - With embeddings enabled and `generation.no_raw_history: true` (default), ashlet sends **only semantically relevant** history commands (not a raw recent-history window).
    - When embeddings are disabled, it may fall back to sending a **recent commands** window.
- **History redaction**: In shell history only, environment variable references (`$SECRET`, `${API_KEY}`) and assignments (`TOKEN=abc`) are redacted before being sent. Safe variables like `$HOME`, `$PATH`, and `$PWD` are preserved.
- **IMPORTANT: Your current input is not redacted.** If you are typing sensitive content, press `Escape` to enable **PRIVATE MODE** until the next prompt (`Enter` / `Ctrl`+`C`). You will see `㊙ PRIVATE MODE ACTIVE - no input sent to AI` below your prompt.
  ![A screenshot of how Private Mode enabled looks like](https://github.com/Paranoid-AF/ashlet/blob/master/.assets/readme/private-mode.png?raw=true)
- **Local-only IPC**: The shell client and daemon communicate over a Unix domain socket. Nothing is sent over the network except API calls to your configured provider.
- **Telemetry**: When `telemetry.openrouter` is `true` (default), OpenRouter attribution headers are sent. Set it to `false` to disable.

## Keybindings

| Key                              | Action                                                     |
| -------------------------------- | ---------------------------------------------------------- |
| `Tab`                            | Accept the displayed suggestion                            |
| `Shift`+`Tab`                    | Fall through to default Zsh completion                     |
| `Shift`+`Left` / `Shift`+`Right` | Browse between candidates                                  |
| `Escape`                         | Enable PRIVATE MODE (stop sending input) until next prompt |

## Troubleshooting

- **No suggestions appear**
  - Ensure the daemon is running: `brew services list` (or start it with `brew services start ashlet`)
  - If you built from source, run `./ashletd` and watch logs for errors
- **`Tab` doesn’t accept the suggestion**
  - Make sure `ashlet.zsh` is sourced in your `~/.zshrc`, then restart your shell
  - If `Tab` is bound by another plugin, you can still access regular Zsh completion via `Shift`+`Tab`
- **Missing dependencies (`jq` / `socat`)**
  - Install them: `brew install jq socat`
- **Socket / connection problems**
  - If you override the socket path, confirm `ASHLET_SOCKET` points to the same location for both shell + daemon
- **API auth / request failures**
  - Set `ASHLET_GENERATION_API_KEY` (or run `ashlet` to create/edit `~/.config/ashlet/config.json`)
- **Accidentally sending something sensitive**
  - Use `Escape` to enable PRIVATE MODE before typing secrets, prventing sending requests (current input is not redacted in requests)

## Configuration

After enabling it in your shell, you can run `ashlet` to generate your configuration files.

This includes:

- `config.json`: General configuration, such as API base URL, your API key, and the model name.
- `prompt.md`: Your custom prompt (Go `text/template`). See the default prompt at: [DEFAULT PROMPT](https://github.com/Paranoid-AF/ashlet/blob/master/default/default_prompt.md).

### config.json

Config lives at `~/.config/ashlet/config.json`:

```json
{
  "version": 1,
  "generation": {
    "base_url": "https://openrouter.ai/api/v1",
    "api_key": "",
    "api_type": "responses",
    "model": "mistralai/codestral-2508",
    "max_tokens": 120,
    "temperature": 0.3,
    "no_raw_history": true
  },
  "embedding": {
    "base_url": "https://openrouter.ai/api/v1",
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

Embeddings are optional. When disabled, ashlet uses recency-only history (no semantic search).

#### API Types

- `"responses"` (default) — OpenAI Responses API (`POST /responses`). Works with OpenRouter.
- `"chat_completions"` — Chat Completions format (`POST /chat/completions`). Use this for Ollama or other local providers.

#### Alternative Ways

You can override some `config.json` values via environment variables.

| Config key            | Priority                                                                   |
| --------------------- | -------------------------------------------------------------------------- |
| `generation.base_url` | `$ASHLET_GENERATION_API_BASE_URL` > `generation.base_url` in `config.json` |
| `generation.api_key`  | `$ASHLET_GENERATION_API_KEY` > `generation.api_key` in `config.json`       |
| `generation.model`    | `$ASHLET_GENERATION_MODEL` > `generation.model` in `config.json`           |
| `embedding.base_url`  | `$ASHLET_EMBEDDING_API_BASE_URL` > `embedding.base_url` in `config.json`   |
| `embedding.api_key`   | `$ASHLET_EMBEDDING_API_KEY` > `embedding.api_key` in `config.json`         |
| `embedding.model`     | `$ASHLET_EMBEDDING_MODEL` > `embedding.model` in `config.json`             |

### `prompt.md`

Prompt lives at `~/.config/ashlet/prompt.md`. It uses Go `text/template` syntax. See the default prompt at: [DEFAULT PROMPT](https://github.com/Paranoid-AF/ashlet/blob/master/default/default_prompt.md) for template variables and format.

### Shell Environment Variables

| Variable                | Default | Description                          |
| ----------------------- | ------- | ------------------------------------ |
| `ASHLET_SOCKET`         | auto    | Override the Unix socket path        |
| `ASHLET_MAX_CANDIDATES` | `4`     | Max suggestions per request          |
| `ASHLET_MIN_INPUT`      | `2`     | Minimum characters before requesting |
| `ASHLET_DELAY`          | `0.05`  | Debounce delay (seconds)             |

## Architecture

```
shell/          Zsh client — captures input, communicates over Unix socket, renders suggestions
generate/       Completion engine — context gathering, API inference, candidate parsing
index/          History indexing and embedding via API
serve/          Daemon entry point and Unix socket server (ashletd)
repl/           Interactive test REPL with cursor tracking (dev-only, not distributed)
default/        Embedded default config and prompt template
ashlet.go       Shared IPC types (Request, Response, Error)
config.go       Configuration types and path resolution
```

Dependency graph: `root (ashlet) <- index <- generate <- serve`

## Development

```bash
make bootstrap    # Download Go dependencies
make build        # Build ashletd
make test         # Go tests + shell tests (bats)
make lint         # go vet + staticcheck + shellcheck
make format       # gofmt + shfmt
make repl         # Build and run the test REPL (dev-only)
```

### Running Locally

Open two terminals to test end-to-end:

**Terminal 1 — daemon:**

```bash
./_run.sh            # build ashletd and start it
./_run.sh --verbose  # same, with request/response logging
```

**Terminal 2 — shell client:**

```bash
./shell/_run.sh
```

This launches an interactive zsh session with ashlet pre-loaded. Type commands to see completions.

### Test REPL

For testing completions without the daemon or shell client:

```bash
make repl             # interactive, TOML output on screen
make repl > log.toml  # save structured output to file
```

The REPL calls the completion engine directly with raw terminal cursor tracking. Each submission outputs structured TOML (context gathered, request, response). Use `:cwd <path>` to change directory, `:quit` to exit. Embeddings are cached to `.cache/` in the project root for fast subsequent runs (REPL-only — the daemon does not use disk cache).

## Why Name It `ashlet`?

**A**I/**A**utocomplete **Sh**ell Script**let** (and a reference to `Ashley`, to make it a bit more _anthropomorphic_).

## License

See [LICENSE](LICENSE) for details.
