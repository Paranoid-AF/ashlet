#!/usr/bin/env zsh
# ashlet — Zsh integration
# Source this file in your .zshrc: source /path/to/ashlet/shell/ashlet.zsh

ASHLET_SCRIPT_DIR="${0:A:h}"

# Load shared client logic
source "${ASHLET_SCRIPT_DIR}/client.sh"

# Zle widget: request completion from daemon and insert into buffer
_ashlet_complete() {
  local input="$BUFFER"
  local cursor_pos="$CURSOR"
  local cwd="$PWD"
  local session_id="$$"

  local response
  response="$(_ashlet_request "$input" "$cursor_pos" "$cwd" "$session_id")"

  if [[ -n "$response" ]]; then
    local completion
    completion="$(_ashlet_parse_completion "$response")"
    if [[ -n "$completion" ]]; then
      BUFFER="${BUFFER[1,$CURSOR]}${completion}${BUFFER[$((CURSOR+1)),-1]}"
      CURSOR=$(( CURSOR + ${#completion} ))
    fi
  fi

  zle redisplay
}

zle -N _ashlet_complete

# Bind to Ctrl+Space by default (customizable)
bindkey '^ ' _ashlet_complete

# Precmd hook: notify daemon of command completion for history tracking
_ashlet_precmd() {
  # TODO: send last command status to daemon for history indexing
  :
}

autoload -Uz add-zsh-hook
add-zsh-hook precmd _ashlet_precmd
