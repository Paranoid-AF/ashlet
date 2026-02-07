#!/usr/bin/env zsh
# init.zsh - Entry point for ashlet autocomplete
# Loads zsh modules and sources all component files

# =============================================================================
# Module Loading
# =============================================================================

# Load required zsh modules
zmodload -F zsh/system b:sysopen p:sysparams 2>/dev/null || {
    print 'ashlet: failed to load zsh/system module' >&2
    return 1
}

zmodload -F zsh/zselect b:zselect 2>/dev/null || {
    print 'ashlet: failed to load zsh/zselect module' >&2
    return 1
}

# Autoload hook registration function
builtin autoload -RUz add-zle-hook-widget 2>/dev/null || {
    print 'ashlet: failed to autoload add-zle-hook-widget (requires zsh 5.3+)' >&2
    return 1
}

# =============================================================================
# Runtime Dependency Check
# =============================================================================

if ! (( $+commands[socat] )); then
    print 'ashlet: missing required dependency: socat' >&2
    return 1
fi
if ! (( $+commands[jq] )); then
    print 'ashlet: missing required dependency: jq' >&2
    return 1
fi

# =============================================================================
# Configuration
# =============================================================================

typeset -gi ASHLET_MAX_CANDIDATES=${ASHLET_MAX_CANDIDATES:-4}
typeset -gi ASHLET_MIN_INPUT=${ASHLET_MIN_INPUT:-2}
typeset -gF ASHLET_DELAY=${ASHLET_DELAY:-0.05}

# =============================================================================
# Source Component Files
# =============================================================================

# Get base directory (parent of autocomplete/)
local basedir="${0:A:h:h}"

# Source client modules
source "${basedir}/client/socket.zsh" || return 1
source "${basedir}/client/request.zsh" || return 1
source "${basedir}/client/response.zsh" || return 1

# Source preferences module (provides 'ashlet' command)
# Non-fatal: preferences TUI is optional, autocomplete should work without it
source "${basedir}/preferences/main.zsh" || print 'ashlet: preferences module failed to load' >&2

# Source autocomplete modules
source "${0:A:h}/state.zsh" || return 1
source "${0:A:h}/display.zsh" || return 1
source "${0:A:h}/async.zsh" || return 1
source "${0:A:h}/hooks.zsh" || return 1
source "${0:A:h}/widgets.zsh" || return 1
source "${0:A:h}/keybindings.zsh" || return 1

# =============================================================================
# Initialization
# =============================================================================

# Register ZLE hooks
.ashlet:register-hooks

# Register keybindings
.ashlet:register-keybindings

# Validate config via daemon (non-fatal, non-blocking)
if .ashlet:socket-exists; then
    local _ashlet_warnings
    _ashlet_warnings=$(print -r -- '{"action":"validate"}' | socat -t2 - "UNIX-CONNECT:$(.ashlet:socket-path)" 2>/dev/null | jq -r '.warnings[]? // empty' 2>/dev/null)
    if [[ -n "$_ashlet_warnings" ]]; then
        print -r -- "ashlet: $_ashlet_warnings" >&2
    fi
fi
