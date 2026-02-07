#!/usr/bin/env bats
# test_run.bats — Tests for _run.sh debug launcher (zsh only)

setup() {
    RUN_SCRIPT="${BATS_TEST_DIRNAME}/../_run.sh"
}

# =============================================================================
# Script Structure Tests
# =============================================================================

@test "_run.sh: script exists and is executable" {
    [ -f "$RUN_SCRIPT" ]
    [ -x "$RUN_SCRIPT" ]
}

@test "_run.sh: contains required functions" {
    run grep -q "_check_deps" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]

    run grep -q "_launch_zsh" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: uses ZDOTDIR for zsh" {
    run grep -q "ZDOTDIR" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

# =============================================================================
# Dependency Check Tests
# =============================================================================

@test "_run.sh: checks for zsh" {
    run grep -q "zsh" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: checks for socat" {
    run grep -q "socat" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: checks for jq" {
    run grep -q "jq" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: checks for llama-server" {
    run grep -q "llama-server" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: offers Homebrew install on macOS" {
    run grep -q "brew install" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

# =============================================================================
# Zsh-only Tests
# =============================================================================

@test "_run.sh: sources ashlet.zsh" {
    run grep -q "ashlet.zsh" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: does not reference ashlet.bash" {
    run grep -q "ashlet.bash" "$RUN_SCRIPT"
    [ "$status" -ne 0 ]
}

@test "_run.sh: creates temporary rc files" {
    run grep -q "mktemp" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: cleans up temp files on exit" {
    run grep -q "trap" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

# =============================================================================
# Error Handling Tests
# =============================================================================

@test "_run.sh: uses set -euo pipefail" {
    run grep -q "set -euo pipefail" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

# =============================================================================
# User Experience Tests
# =============================================================================

@test "_run.sh: prints shell ready message" {
    run grep -q "shell ready" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}

@test "_run.sh: sources user's existing zshrc" {
    run grep -q "\.zshrc" "$RUN_SCRIPT"
    [ "$status" -eq 0 ]
}
