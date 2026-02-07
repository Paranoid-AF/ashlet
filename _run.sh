#!/bin/sh
# ashlet — Build and run the ashletd daemon
# Builds latest and starts the daemon
#
# Usage: _run.sh [-v|--verbose]
#   -v, --verbose   Enable verbose logging (log every request/response)

ASHLET_DIR="$(cd "$(dirname "$0")" && pwd)"

# --- Dependency check ---
# Required: llama-server (inference engine from llama.cpp)
check_deps() {
  if command -v llama-server >/dev/null 2>&1; then
    return 0
  fi

  printf 'ashlet: missing dependency: llama-server (from llama.cpp)\n' >&2

  case "$(uname -s)" in
    Darwin)
      if command -v brew >/dev/null 2>&1; then
        printf 'Install with Homebrew? [Y/n] ' >&2
        read -r reply
        case "$reply" in
          [nN]) printf 'ashlet: install llama.cpp manually and retry.\n' >&2; exit 1 ;;
        esac
        brew install llama.cpp
        if ! command -v llama-server >/dev/null 2>&1; then
          printf 'ashlet: llama-server still not found after install.\n' >&2
          exit 1
        fi
      else
        printf 'ashlet: install Homebrew (https://brew.sh) or install llama.cpp manually.\n' >&2
        exit 1
      fi
      ;;
    *)
      printf 'ashlet: install llama.cpp using your package manager and retry.\n' >&2
      exit 1
      ;;
  esac
}

check_deps

# Parse arguments
VERBOSE=""
for arg in "$@"; do
  case "$arg" in
    -v|--verbose) VERBOSE="--verbose" ;;
  esac
done

cd "$ASHLET_DIR" || exit 1

echo "Building ashletd..."
if ! make build; then
  echo "Error: Build failed" >&2
  exit 1
fi

echo "Starting ashletd..."
exec ./ashletd $VERBOSE
