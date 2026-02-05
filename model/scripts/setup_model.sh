#!/usr/bin/env bash
# ashlet — Model setup script
# Downloads and prepares GGUF model files for ashlet.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODEL_DIR="${SCRIPT_DIR}/../models"

mkdir -p "$MODEL_DIR"

echo "ashlet model setup"
echo "=================="
echo ""
echo "Model directory: $MODEL_DIR"
echo ""

# TODO: define default model and download URL
# For now, print instructions for manual setup
echo "Automatic model download is not yet configured."
echo ""
echo "To set up manually:"
echo "  1. Download a GGUF model (e.g., from Hugging Face)"
echo "  2. Place the .gguf file in: $MODEL_DIR"
echo ""
echo "Recommended models for shell completion:"
echo "  - A small (1-3B parameter) instruction-tuned model"
echo "  - GGUF Q4_K_M quantization for good quality/speed balance"
