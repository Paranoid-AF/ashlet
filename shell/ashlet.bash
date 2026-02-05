#!/usr/bin/env bash
# ashlet — Bash integration
# Source this file in your .bashrc: source /path/to/ashlet/shell/ashlet.bash

ASHLET_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load shared client logic
source "${ASHLET_SCRIPT_DIR}/client.sh"

# Readline completion function
_ashlet_complete() {
  local input="${READLINE_LINE}"
  local cursor_pos="${READLINE_POINT}"
  local cwd="$PWD"
  local session_id="$$"

  local response
  response="$(_ashlet_request "$input" "$cursor_pos" "$cwd" "$session_id")"

  if [[ -n "$response" ]]; then
    local completion
    completion="$(_ashlet_parse_completion "$response")"
    if [[ -n "$completion" ]]; then
      READLINE_LINE="${READLINE_LINE:0:$READLINE_POINT}${completion}${READLINE_LINE:$READLINE_POINT}"
      READLINE_POINT=$(( READLINE_POINT + ${#completion} ))
    fi
  fi
}

# Bind to Ctrl+Space by default (customizable)
bind -x '"\C- ": _ashlet_complete'
