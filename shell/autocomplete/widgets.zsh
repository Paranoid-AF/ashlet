#!/usr/bin/env zsh
# widgets.zsh - User ZLE widgets for ashlet (apply, navigate, dismiss, history)

# =============================================================================
# Apply Candidate (TAB)
# =============================================================================

.ashlet:apply-tab() {
    # Only apply if we have candidates, at history tip, and not dismissed
    if (( _ashlet_candidate_count > 0 && _ashlet_at_history_tip && ! _ashlet_dismissed )); then
        local completion cursor_pos
        completion="$(.ashlet:parse-candidate-at "$_ashlet_response" "$_ashlet_browse_index")"
        cursor_pos="$(.ashlet:parse-candidate-cursor-at "$_ashlet_response" "$_ashlet_browse_index")"

        # Apply if candidate is valid (server handles relevance, including typo fixes)
        if [[ -n "$completion" ]] && .ashlet:candidate-valid "$completion"; then
            # Replace buffer with completion
            BUFFER="$completion"

            # Set cursor position
            if [[ -n "$cursor_pos" ]] && [[ "$cursor_pos" =~ ^[0-9]+$ ]]; then
                CURSOR=$cursor_pos
            else
                CURSOR=${#BUFFER}
            fi

            # Clear state (transition to IDLE)
            .ashlet:clear-candidates

            return
        fi
    fi

    # Fall through to default completion
    zle expand-or-complete
}
zle -N .ashlet:apply-tab

# =============================================================================
# Navigate Candidates (Shift+Arrow)
# =============================================================================

.ashlet:prev-candidate() {
    if (( _ashlet_candidate_count > 0 && _ashlet_at_history_tip && ! _ashlet_dismissed )); then
        (( _ashlet_browse_index = (_ashlet_browse_index - 1 + _ashlet_candidate_count) % _ashlet_candidate_count ))
        .ashlet:show-candidate
        zle -R
    fi
}
zle -N .ashlet:prev-candidate

.ashlet:next-candidate() {
    if (( _ashlet_candidate_count > 0 && _ashlet_at_history_tip && ! _ashlet_dismissed )); then
        (( _ashlet_browse_index = (_ashlet_browse_index + 1) % _ashlet_candidate_count ))
        .ashlet:show-candidate
        zle -R
    fi
}
zle -N .ashlet:next-candidate

# =============================================================================
# Dismiss (ESC)
# =============================================================================

.ashlet:dismiss() {
    _ashlet_dismissed=1
    .ashlet:clear-candidates
    zle -R
}
zle -N .ashlet:dismiss

# =============================================================================
# History Navigation (Up/Down)
# =============================================================================

.ashlet:history-up() {
    .ashlet:clear-candidates
    _ashlet_at_history_tip=0
    zle up-line-or-history
}
zle -N .ashlet:history-up

.ashlet:history-down() {
    zle down-line-or-history

    # Check if we're back at the tip
    # ZLE doesn't expose history position directly, but after going down
    # we might be back at the tip. For simplicity, always re-enable candidates.
    .ashlet:clear-candidates
    _ashlet_at_history_tip=1
}
zle -N .ashlet:history-down
