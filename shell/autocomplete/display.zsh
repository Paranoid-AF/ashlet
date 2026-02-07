#!/usr/bin/env zsh
# display.zsh - POSTDISPLAY and region_highlight management for ashlet

# =============================================================================
# Display Formatting
# =============================================================================

# Format candidate display line
# Usage: .ashlet:format-candidate <completion> <index> <count> [cursor_pos]
.ashlet:format-candidate() {
    local completion="$1"
    local index="$2"
    local count="$3"
    local cursor_pos="${4:-}"

    # Insert cursor marker if cursor_pos is set
    local display_text="$completion"
    if [[ -n "$cursor_pos" ]] && [[ "$cursor_pos" =~ ^[0-9]+$ ]]; then
        if (( cursor_pos < ${#completion} )); then
            display_text="${completion:0:$cursor_pos}█${completion:$cursor_pos}"
        fi
    fi

    # Format: (N/M) completion text
    print -rn -- "($((index + 1))/$count) $display_text"
}

# Get hint text for keybindings
.ashlet:hint-text() {
    print -rn -- '[↹] apply | [⇧]+[←/→] navigate'
}

# =============================================================================
# POSTDISPLAY Management
# =============================================================================

# Show candidate line below prompt
.ashlet:show-candidate() {
    # Guard: Don't show if dismissed or not at history tip
    if (( _ashlet_dismissed || ! _ashlet_at_history_tip )); then
        .ashlet:clear-display
        return
    fi

    # Guard: Don't show if no candidates
    if (( _ashlet_candidate_count == 0 )); then
        .ashlet:clear-display
        return
    fi

    local completion cursor_pos
    completion="$(.ashlet:parse-candidate-at "$_ashlet_response" "$_ashlet_browse_index")"
    cursor_pos="$(.ashlet:parse-candidate-cursor-at "$_ashlet_response" "$_ashlet_browse_index")"

    if [[ -z "$completion" ]]; then
        .ashlet:clear-display
        return
    fi

    # Validate candidate (server is trusted for relevance, including typo fixes)
    if ! .ashlet:candidate-valid "$completion"; then
        .ashlet:clear-display
        return
    fi

    # Format the display line
    local formatted hint
    formatted="$(.ashlet:format-candidate "$completion" "$_ashlet_browse_index" "$_ashlet_candidate_count" "$cursor_pos")"
    hint="$(.ashlet:hint-text)"

    # Calculate padding for right-aligned hint
    local -i term_width=${COLUMNS:-80}
    local -i left_len=${#formatted}
    local -i hint_len=${#hint}
    local -i padding=$((term_width - left_len - hint_len - 1))  # -1 for newline
    (( padding < 2 )) && padding=2  # Minimum 2 spaces between

    # Build POSTDISPLAY with newline prefix and calculated spacing
    local spaces="${(l:$padding:)}"
    POSTDISPLAY=$'\n'"${formatted}${spaces}${hint}"

    # Apply highlighting
    .ashlet:apply-highlights "$hint"
}

# Show private mode indicator below prompt
.ashlet:show-private-mode() {
    local msg='㊙ PRIVATE MODE ACTIVE - no input sent to AI'
    POSTDISPLAY=$'\n'"$msg"

    # Highlight the message in yellow/orange
    region_highlight=("${(@)region_highlight:#*ashlet*}")
    local -i base_offset=${#BUFFER}
    local -i msg_start=$((base_offset + 1))  # +1 for newline
    local -i msg_end=$((msg_start + ${#msg}))
    region_highlight+=("${msg_start} ${msg_end} fg=208 ashlet")
}

# Apply region_highlight for colors
.ashlet:apply-highlights() {
    local hint="$1"

    # Remove old ashlet highlights
    region_highlight=("${(@)region_highlight:#*ashlet*}")

    # Calculate offsets for highlighting (POSTDISPLAY starts after BUFFER)
    local -i base_offset=${#BUFFER}
    local -i newline_len=1

    # Gray for hints (fg=242) - at the end of POSTDISPLAY
    local -i hint_len=${#hint}
    local -i hint_start=$((base_offset + ${#POSTDISPLAY} - hint_len))
    local -i hint_end=$((base_offset + ${#POSTDISPLAY}))
    region_highlight+=("${hint_start} ${hint_end} fg=242 ashlet")
}
