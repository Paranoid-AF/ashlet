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

# Check if ashletd is running as a brew service before stopping
WAS_RUNNING_BY_BREW=false
if command -v brew >/dev/null 2>&1 && pgrep -f '/opt/homebrew/.*ashletd' >/dev/null 2>&1; then
  WAS_RUNNING_BY_BREW=true
fi

# Gracefully stop the brew service; fall back to pkill if it hangs.
if [ "$WAS_RUNNING_BY_BREW" = true ]; then
  brew services stop ashlet >/dev/null 2>&1 &
  BREW_PID=$!
  ( sleep 3 && kill $BREW_PID 2>/dev/null && pkill -f ashletd 2>/dev/null ) &
  wait $BREW_PID 2>/dev/null || true
else
  pkill -f ashletd 2>/dev/null
fi
sleep 1

if [ "$WAS_RUNNING_BY_BREW" = true ]; then
  trap 'brew services start ashlet >/dev/null 2>&1' EXIT
fi

./ashletd $VERBOSE
