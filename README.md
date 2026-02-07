# ashlet

![Live Demo](https://github.com/Paranoid-AF/ashlet/blob/master/.assets/readme/demo.gif?raw=true)

AI-powered shell auto-completion for Zsh. Suggestions appear as you type, powered by any OpenAI-compatible API.

ashlet runs a lightweight daemon (`ashletd`) that gathers context from your shell — working directory, command history, git status, project manifests — and sends it to an inference API. Candidates are streamed back and displayed inline below your prompt.

## Install

### Homebrew

```bash
brew install paranoid-af/tap/ashlet
```

Add to `~/.zshrc`:

```bash
source $(brew --prefix)/share/ashlet/ashlet.zsh
```

Start the daemon (runs automatically on login):

```bash
brew services start ashlet
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

## Keybindings

| Key                      | Action                                 |
| ------------------------ | -------------------------------------- |
| Tab                      | Accept the displayed suggestion        |
| Shift+Tab                | Fall through to default Zsh completion |
| Shift+Left / Shift+Right | Browse between candidates              |
| Escape                   | Dismiss suggestions until next input   |

## Configuration

Config lives at `~/.config/ashlet/config.json` (created on first run of `ashlet`):

```json
{
  "version": 2,
  "generation": {
    "base_url": "https://openrouter.ai/api/v1",
    "api_key": "",
    "api_type": "responses",
    "model": "mistralai/codestral-2508",
    "max_tokens": 120,
    "temperature": 0.3
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

### API Key Resolution

| Key        | Priority                                                      |
| ---------- | ------------------------------------------------------------- |
| Generation | `$ASHLET_GENERATION_API_KEY` > `generation.api_key` in config |
| Embedding  | `$ASHLET_EMBEDDING_API_KEY` > `embedding.api_key` in config   |

Embeddings are optional — when disabled, history ranking falls back to recency.

### API Types

- `"responses"` (default) — OpenAI Responses API (`POST /responses`). Works with OpenRouter.
- `"chat_completions"` — Chat Completions format (`POST /chat/completions`). Use this for Ollama or other local providers.

### Shell Environment Variables

| Variable                | Default | Description                          |
| ----------------------- | ------- | ------------------------------------ |
| `ASHLET_SOCKET`         | auto    | Override the Unix socket path        |
| `ASHLET_MAX_CANDIDATES` | `4`     | Max suggestions per request          |
| `ASHLET_MIN_INPUT`      | `2`     | Minimum characters before requesting |
| `ASHLET_DELAY`          | `0.05`  | Debounce delay (seconds)             |

### Custom Prompt

Place a custom system prompt at `~/.config/ashlet/prompt.md`. It uses Go `text/template` syntax. See `default/default_prompt.md` for the template variables and format.

## Architecture

```
shell/          Zsh client — captures input, communicates over Unix socket, renders suggestions
generate/       Completion engine — context gathering, API inference, candidate parsing
index/          History indexing and embedding via API
serve/          Daemon entry point and Unix socket server (ashletd)
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
```

## License

See [LICENSE](LICENSE) for details.
