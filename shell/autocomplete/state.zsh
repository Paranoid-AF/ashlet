#!/usr/bin/env zsh
# state.zsh - State variables and management for ashlet autocomplete

# =============================================================================
# State Variables
# =============================================================================

typeset -g  _ashlet_response=""          # Raw JSON response
typeset -gi _ashlet_candidate_count=0    # Number of candidates in response
typeset -gi _ashlet_browse_index=0       # Currently highlighted candidate (0-indexed)
typeset -gi _ashlet_dismissed=0          # True if user pressed ESC
typeset -gi _ashlet_private_mode=0       # True if private mode active (ESC)
typeset -gi _ashlet_at_history_tip=1     # True if buffer is at the latest history entry
typeset -g  _ashlet_lbuffer=""           # Saved LBUFFER for state comparison
typeset -g  _ashlet_rbuffer=""           # Saved RBUFFER for state comparison
typeset -gi _ashlet_next_req_id=1        # Counter for outgoing request IDs
typeset -gi _ashlet_last_resp_id=0       # Highest response ID accepted
typeset -gi _ashlet_wait_fd=0            # File descriptor for debounce timer
typeset -gi _ashlet_complete_fd=0        # File descriptor for async completion
typeset -gi _ashlet_saved_stdout=0       # Saved stdout fd for preexec restoration
typeset -gi _ashlet_saved_stderr=0       # Saved stderr fd for preexec restoration

# =============================================================================
# State Management Functions
# =============================================================================

# Save current state for comparison
.ashlet:save-state() {
    typeset -g _ashlet_lbuffer="$LBUFFER"
    typeset -g _ashlet_rbuffer="$RBUFFER"
}

# Check if state (buffer + cursor) is unchanged
.ashlet:same-state() {
    [[ -v _ashlet_lbuffer && $_ashlet_lbuffer == $LBUFFER &&
       -v _ashlet_rbuffer && $_ashlet_rbuffer == $RBUFFER ]]
}

# Reset all state (called on new prompt)
.ashlet:reset-state() {
    _ashlet_response=""
    _ashlet_candidate_count=0
    _ashlet_browse_index=0
    _ashlet_dismissed=0
    _ashlet_private_mode=0
    _ashlet_at_history_tip=1
    _ashlet_lbuffer=""
    _ashlet_rbuffer=""
    _ashlet_wait_fd=0
    _ashlet_complete_fd=0
    POSTDISPLAY=$'\n'
    # Remove any ashlet highlights
    region_highlight=("${(@)region_highlight:#*ashlet*}")
}

# Clear candidates but keep request tracking
# Reserves a blank row to prevent input buffer position shift
.ashlet:clear-candidates() {
    _ashlet_response=""
    _ashlet_candidate_count=0
    _ashlet_browse_index=0
    POSTDISPLAY=$'\n'
    region_highlight=("${(@)region_highlight:#*ashlet*}")
}

# Clear display only (keep response data for stale-while-revalidate)
# Reserves a blank row to prevent input buffer position shift
.ashlet:clear-display() {
    POSTDISPLAY=$'\n'
    region_highlight=("${(@)region_highlight:#*ashlet*}")
}

# Check if candidate is valid for current buffer
# Note: We trust the server to return relevant candidates, including typo corrections
# The request ID ordering already handles stale responses
.ashlet:candidate-valid() {
    local completion="$1"
    # Always valid - server is responsible for relevance
    [[ -n "$completion" ]]
}
