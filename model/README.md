# ashlet — Model Layer

Wraps llama.cpp for local AI inference. Handles both embedding generation (for history indexing) and text generation (for completions).

## Runtime

- **Engine**: llama.cpp
- **Format**: GGUF
- **Acceleration**: Apple Silicon Metal (GPU), CPU fallback

## Setup

```bash
make setup    # Clone llama.cpp and download default model
make build    # Build llama.cpp
```

## Models

Model files are stored in `models/` (gitignored). Use the setup script to download:

```bash
./scripts/setup_model.sh
```

## Architecture

- **embedding/embed.go** — Go interface for generating embeddings from shell commands/context
- **inference/infer.go** — Go interface for text generation (completion suggestions)
- **scripts/setup_model.sh** — Downloads and prepares GGUF model files
