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
exec ./ashletd $VERBOSE
