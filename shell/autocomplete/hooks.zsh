#!/usr/bin/env zsh
# hooks.zsh - ZLE hook widgets for ashlet

# =============================================================================
# Hook Widgets
# =============================================================================

# Called on new prompt (line-init hook)
.ashlet:line-init() {
    .ashlet:reset-state
    return 0
}
zle -N .ashlet:line-init

# Called before each redraw (line-pre-redraw hook)
# Detects buffer/cursor changes and triggers async fetch
.ashlet:line-pre-redraw() {
    # Check if state has changed (buffer content OR cursor position)
    if ! .ashlet:same-state; then
        # Private mode: don't fetch, don't clear dismissed, just update state + display
        if (( _ashlet_private_mode )); then
            .ashlet:save-state
            .ashlet:show-private-mode
            zle -R
            return 0
        fi

        # Clear dismissed state on any change
        if (( _ashlet_dismissed )); then
            _ashlet_dismissed=0
        fi

        # Only fetch if at history tip and buffer meets minimum length
        if (( _ashlet_at_history_tip && ${#BUFFER} >= ASHLET_MIN_INPUT )); then
            # Trigger async fetch with debounce
            .ashlet:trigger-async
        elif (( ${#BUFFER} < ASHLET_MIN_INPUT )); then
            # Buffer too short - dismiss candidates and cancel pending requests
            .ashlet:cleanup-async
            .ashlet:clear-candidates
            .ashlet:save-state
        else
            # Just save state without fetching (not at history tip)
            .ashlet:save-state
        fi
    fi

    return 0
}
zle -N .ashlet:line-pre-redraw

# Called when line is finished (line-finish hook)
# Fully clears POSTDISPLAY (no reserved blank row) before execution
.ashlet:line-finish() {
    .ashlet:cleanup-async
    POSTDISPLAY=""
    region_highlight=("${(@)region_highlight:#*ashlet*}")
    return 0
}
zle -N .ashlet:line-finish

# =============================================================================
# Regular Hooks (run outside ZLE — safe for external commands)
# =============================================================================

# Warm the directory context cache before each prompt.
# This must NOT run inside a ZLE widget because it calls external commands
# (socat). External process forks from ZLE widgets break terminal state.
.ashlet:precmd-hook() {
    .ashlet:context-request "$PWD"
}

# Restore stdout/stderr before command execution.
# Async fd handling in ZLE widgets (sysopen, process substitution, exec {fd}<&-)
# can cause standard fds to be redirected away from the terminal. This ensures
# commands always have working stdout and stderr.
.ashlet:preexec-hook() {
    [[ -t 1 ]] || exec 1>/dev/tty
    [[ -t 2 ]] || exec 2>/dev/tty
}

# =============================================================================
# Hook Registration
# =============================================================================

# Register hooks using add-zle-hook-widget for proper chaining
.ashlet:register-hooks() {
    add-zle-hook-widget line-init      .ashlet:line-init
    add-zle-hook-widget line-pre-redraw .ashlet:line-pre-redraw
    add-zle-hook-widget line-finish    .ashlet:line-finish

    # precmd/preexec are regular zsh hooks (not ZLE), safe for external commands
    autoload -Uz add-zsh-hook
    add-zsh-hook precmd .ashlet:precmd-hook
    add-zsh-hook preexec .ashlet:preexec-hook
}
