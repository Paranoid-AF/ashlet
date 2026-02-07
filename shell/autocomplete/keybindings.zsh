#!/usr/bin/env zsh
# keybindings.zsh - Keybinding declarations for ashlet

# =============================================================================
# Keybindings
# =============================================================================

.ashlet:register-keybindings() {
    # TAB - apply candidate (with fallback to default completion)
    bindkey '^I' .ashlet:apply-tab

    # Shift+TAB - always use default completion
    bindkey '^[[Z' expand-or-complete

    # Shift+Left - previous candidate
    bindkey '^[[1;2D' .ashlet:prev-candidate

    # Shift+Right - next candidate
    bindkey '^[[1;2C' .ashlet:next-candidate

    # ESC - dismiss (note: may conflict with vi-mode)
    bindkey '^[' .ashlet:dismiss

    # Up arrow - history navigation
    bindkey '^[[A' .ashlet:history-up

    # Down arrow - history navigation
    bindkey '^[[B' .ashlet:history-down
}
