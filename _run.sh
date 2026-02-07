#!/bin/sh
# ashlet — Build and run the ashletd daemon
# Builds latest and starts the daemon
#
# Usage: _run.sh [-v|--verbose]
#   -v, --verbose   Enable verbose logging (log every request/response)

ASHLET_DIR="$(cd "$(dirname "$0")" && pwd)"

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

# Stop running service for debugging, and restart it on exit
if command -v brew >/dev/null 2>&1 \
  && [ "$(brew services list 2>/dev/null | awk '/^ashlet/ { print $2 }')" != "none" ]; then
  echo "Stopping enabled ashlet background service..."
  brew services stop ashlet >/dev/null 2>&1 &
  BREW_PID=$!
  sleep 2 && kill $BREW_PID 2>/dev/null &
  wait $BREW_PID 2>/dev/null || true
  trap 'echo "Restarting enabled ashlet background service..."; brew services start ashlet >/dev/null 2>&1' EXIT
fi

./ashletd $VERBOSE
