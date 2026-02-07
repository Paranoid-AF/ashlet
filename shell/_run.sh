#!/usr/bin/env bash
# _run.sh - Debug launcher for ashlet shell integration (zsh only)
# Starts an interactive zsh shell with ashlet pre-loaded for development/testing

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# =============================================================================
# Dependency Checking
# =============================================================================

_check_command() {
    local cmd="$1"
    command -v "$cmd" >/dev/null 2>&1
}

_offer_brew_install() {
    local pkg="$1"
    if [[ "$(uname)" == "Darwin" ]] && _check_command brew; then
        printf 'Install %s via Homebrew? [y/N] ' "$pkg"
        read -r reply
        if [[ "$reply" =~ ^[Yy]$ ]]; then
            brew install "$pkg"
            return $?
        fi
    fi
    return 1
}

_check_deps() {
    local missing=()

    if ! _check_command zsh; then
        printf 'ashlet: zsh not found\n' >&2
        missing+=("zsh")
    fi

    if ! _check_command socat; then
        printf 'ashlet: socat not found\n' >&2
        if ! _offer_brew_install socat; then
            missing+=("socat")
        fi
    fi

    if ! _check_command jq; then
        printf 'ashlet: jq not found\n' >&2
        if ! _offer_brew_install jq; then
            missing+=("jq")
        fi
    fi

    if [[ ${#missing[@]} -gt 0 ]]; then
        printf 'ashlet: missing dependencies: %s\n' "${missing[*]}" >&2
        printf 'Install the missing dependencies using your system package manager and retry.\n' >&2
        exit 1
    fi
}

# =============================================================================
# Launcher
# =============================================================================

_launch_zsh() {
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    # Create custom .zshrc that sources user's rc + ashlet
    cat > "${tmpdir}/.zshrc" <<EOF
# Source user's zshrc if it exists
[[ -f ~/.zshrc ]] && source ~/.zshrc

# Source ashlet
source "${SCRIPT_DIR}/ashlet.zsh"

# Indicator that ashlet is active
printf '\\033[38;5;208mashlet\\033[0m shell ready\\n'
EOF

    # Launch zsh with custom ZDOTDIR
    ZDOTDIR="$tmpdir" exec zsh -i
}

# =============================================================================
# Main
# =============================================================================

main() {
    _check_deps
    printf 'Launching zsh with ashlet...\n'
    _launch_zsh
}

main "$@"
