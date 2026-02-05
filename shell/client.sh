#!/usr/bin/env bash
# ashlet — Shared client logic for shell integrations
# Connects to the ashlet daemon via Unix domain socket, sends context, receives completions.

# Resolve socket path
_ashlet_socket_path() {
  if [[ -n "${ASHLET_SOCKET:-}" ]]; then
    echo "$ASHLET_SOCKET"
  elif [[ -n "${XDG_RUNTIME_DIR:-}" ]]; then
    echo "${XDG_RUNTIME_DIR}/ashlet.sock"
  else
    echo "/tmp/ashlet-$(id -u).sock"
  fi
}

# Send a completion request to the daemon
# Args: input, cursor_pos, cwd, session_id
_ashlet_request() {
  local input="$1"
  local cursor_pos="$2"
  local cwd="$3"
  local session_id="$4"
  local socket_path
  socket_path="$(_ashlet_socket_path)"

  if [[ ! -S "$socket_path" ]]; then
    return 1
  fi

  # Escape input for JSON (basic escaping)
  local escaped_input
  escaped_input="$(printf '%s' "$input" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g')"

  local escaped_cwd
  escaped_cwd="$(printf '%s' "$cwd" | sed 's/\\/\\\\/g; s/"/\\"/g')"

  local request
  request=$(printf '{"input":"%s","cursor_pos":%d,"cwd":"%s","session_id":"%s"}' \
    "$escaped_input" "$cursor_pos" "$escaped_cwd" "$session_id")

  # Send request via socat
  if command -v socat >/dev/null 2>&1; then
    echo "$request" | socat - UNIX-CONNECT:"$socket_path" 2>/dev/null
  else
    return 1
  fi
}

# Parse the completion text from a JSON response
# Args: json_response
_ashlet_parse_completion() {
  local response="$1"

  # Extract completion field from JSON response
  # Uses basic pattern matching to avoid dependency on jq
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$response" | jq -r '.completion // empty'
  else
    # Fallback: simple grep extraction
    printf '%s' "$response" | grep -o '"completion":"[^"]*"' | sed 's/"completion":"//;s/"$//'
  fi
}
