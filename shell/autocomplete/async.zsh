#!/usr/bin/env zsh
# async.zsh - Async I/O with sysopen, zselect, and debounce for ashlet

# =============================================================================
# Debounced Async Trigger
# =============================================================================

# Trigger async fetch with debounce
.ashlet:trigger-async() {
    # Cancel any pending debounce timer
    if (( _ashlet_wait_fd > 2 )); then
        zle -F $_ashlet_wait_fd 2>/dev/null
        exec {_ashlet_wait_fd}<&- 2>/dev/null
        _ashlet_wait_fd=0
    fi

    # Save current state for comparison after delay
    .ashlet:save-state

    # Start debounce timer using sysopen + process substitution
    local fd=0
    if sysopen -r -o cloexec -u fd <(
        # Convert ASHLET_DELAY (seconds) to centiseconds for zselect
        local -i timeout=$(( [#10] 100 * ASHLET_DELAY ))
        zselect -t $timeout 2>/dev/null
        print
    ) 2>/dev/null; then
        _ashlet_wait_fd=$fd
        zle -Fw $fd .ashlet:wait-callback
    else
        # Fallback: fetch immediately if sysopen fails
        .ashlet:fetch-async
    fi
}

# Callback after debounce delay
.ashlet:wait-callback() {
    local -i fd=$1

    # Unregister and close fd (guard: never close standard fds 0-2)
    zle -F $fd 2>/dev/null
    (( fd > 2 )) && exec {fd}<&- 2>/dev/null
    _ashlet_wait_fd=0

    # Abort if keys are queued (user is still typing)
    (( KEYS_QUEUED_COUNT || PENDING )) && return

    # Abort if state has changed during the delay
    .ashlet:same-state || return

    # Proceed with fetch
    .ashlet:fetch-async
}
zle -N .ashlet:wait-callback

# =============================================================================
# Async Fetch
# =============================================================================

# Send async request to daemon
.ashlet:fetch-async() {
    # Cancel any pending fetch
    if (( _ashlet_complete_fd > 2 )); then
        zle -F $_ashlet_complete_fd 2>/dev/null
        exec {_ashlet_complete_fd}<&- 2>/dev/null
        _ashlet_complete_fd=0
    fi

    local req_id=$_ashlet_next_req_id
    (( _ashlet_next_req_id++ ))

    # Launch request in background with sysopen
    local fd=0
    if sysopen -r -o cloexec -u fd <(
        .ashlet:request "$req_id" "$BUFFER" "$CURSOR" "$PWD" "$$"
    ) 2>/dev/null; then
        _ashlet_complete_fd=$fd
        zle -Fw $fd .ashlet:complete-callback
    fi
}

# Callback when response arrives
.ashlet:complete-callback() {
    local -i fd=$1
    local data=""

    # Read all available data
    while IFS= read -r -u $fd line 2>/dev/null; do
        data+="$line"
    done

    # Unregister and close fd (guard: never close standard fds 0-2)
    zle -F $fd 2>/dev/null
    (( fd > 2 )) && exec {fd}<&- 2>/dev/null
    _ashlet_complete_fd=0

    # Validate response
    if [[ -z "$data" ]]; then
        return
    fi

    local resp_id
    resp_id="$(.ashlet:response-id "$data")"

    # Discard stale responses
    if [[ -z "$resp_id" ]] || (( resp_id <= _ashlet_last_resp_id )); then
        return
    fi

    # Check for errors
    if .ashlet:has-error "$data"; then
        return
    fi

    # Abort if state has changed while waiting
    .ashlet:same-state || return

    # Update state atomically
    _ashlet_last_resp_id=$resp_id
    _ashlet_response="$data"
    _ashlet_candidate_count="$(.ashlet:candidate-count "$data")"
    _ashlet_browse_index=0

    # Update display
    .ashlet:show-candidate

    # Force visual refresh
    zle -R
}
zle -N .ashlet:complete-callback

# =============================================================================
# Cleanup
# =============================================================================

# Close all async file descriptors
.ashlet:cleanup-async() {
    if (( _ashlet_wait_fd > 2 )); then
        zle -F $_ashlet_wait_fd 2>/dev/null
        exec {_ashlet_wait_fd}<&- 2>/dev/null
        _ashlet_wait_fd=0
    fi
    if (( _ashlet_complete_fd > 2 )); then
        zle -F $_ashlet_complete_fd 2>/dev/null
        exec {_ashlet_complete_fd}<&- 2>/dev/null
        _ashlet_complete_fd=0
    fi
}
