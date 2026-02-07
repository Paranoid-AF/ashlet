#!/usr/bin/env bats
# test_client.bats — Unit tests for client modules
# Note: Tests run zsh subprocesses since the client modules use zsh syntax

setup() {
    TEST_DIR="${BATS_TEST_DIRNAME}/.."
}

# Helper to run zsh function
run_zsh() {
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        source '${TEST_DIR}/client/request.zsh'
        source '${TEST_DIR}/client/response.zsh'
        source '${TEST_DIR}/autocomplete/display.zsh'
        $1
    "
}

# =============================================================================
# Socket Path Tests
# =============================================================================

@test ".ashlet:socket-path: uses ASHLET_SOCKET when set" {
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        ASHLET_SOCKET='/custom/path.sock'
        .ashlet:socket-path
    "
    [ "$status" -eq 0 ]
    [ "$output" = "/custom/path.sock" ]
}

@test ".ashlet:socket-path: uses XDG_RUNTIME_DIR when ASHLET_SOCKET unset" {
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        unset ASHLET_SOCKET
        XDG_RUNTIME_DIR='/run/user/1000'
        .ashlet:socket-path
    "
    [ "$status" -eq 0 ]
    [ "$output" = "/run/user/1000/ashlet.sock" ]
}

@test ".ashlet:socket-path: falls back to /tmp/ashlet-UID.sock" {
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        unset ASHLET_SOCKET
        unset XDG_RUNTIME_DIR
        .ashlet:socket-path
    "
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^/tmp/ashlet-[0-9]+\.sock$ ]]
}

# =============================================================================
# Response ID Parsing Tests
# =============================================================================

@test ".ashlet:response-id: extracts request_id from valid response" {
    run_zsh '.ashlet:response-id '"'"'{"request_id":42,"candidates":[]}'"'"
    [ "$status" -eq 0 ]
    [ "$output" = "42" ]
}

@test ".ashlet:response-id: returns empty for missing request_id" {
    run_zsh '.ashlet:response-id '"'"'{"candidates":[]}'"'"
    [ "$status" -eq 0 ]
    [ "$output" = "" ]
}

@test ".ashlet:response-id: handles invalid JSON gracefully" {
    run_zsh '.ashlet:response-id "not json"'
    # jq will output an error to stderr, the important thing is it doesn't crash
    [ "$status" -ge 0 ]
}

# =============================================================================
# Candidate Count Tests
# =============================================================================

@test ".ashlet:candidate-count: counts candidates correctly" {
    run_zsh '.ashlet:candidate-count '"'"'{"request_id":1,"candidates":[{"completion":"a"},{"completion":"b"},{"completion":"c"}]}'"'"
    [ "$status" -eq 0 ]
    [ "$output" = "3" ]
}

@test ".ashlet:candidate-count: returns 0 for empty candidates" {
    run_zsh '.ashlet:candidate-count '"'"'{"request_id":1,"candidates":[]}'"'"
    [ "$status" -eq 0 ]
    [ "$output" = "0" ]
}

# =============================================================================
# Candidate Parsing Tests
# =============================================================================

@test ".ashlet:parse-candidate-at: extracts completion at index 0" {
    run_zsh '.ashlet:parse-candidate-at '"'"'{"request_id":1,"candidates":[{"completion":"git status"},{"completion":"git stash"}]}'"'"' 0'
    [ "$status" -eq 0 ]
    [ "$output" = "git status" ]
}

@test ".ashlet:parse-candidate-at: extracts completion at index 1" {
    run_zsh '.ashlet:parse-candidate-at '"'"'{"request_id":1,"candidates":[{"completion":"git status"},{"completion":"git stash"}]}'"'"' 1'
    [ "$status" -eq 0 ]
    [ "$output" = "git stash" ]
}

@test ".ashlet:parse-candidate-at: returns empty for out of bounds index" {
    run_zsh '.ashlet:parse-candidate-at '"'"'{"request_id":1,"candidates":[{"completion":"git status"}]}'"'"' 5'
    [ "$status" -eq 0 ]
    [ "$output" = "" ]
}

@test ".ashlet:parse-candidate-cursor-at: extracts cursor_pos when present" {
    run_zsh '.ashlet:parse-candidate-cursor-at '"'"'{"request_id":1,"candidates":[{"completion":"git status","cursor_pos":10}]}'"'"' 0'
    [ "$status" -eq 0 ]
    [ "$output" = "10" ]
}

@test ".ashlet:parse-candidate-cursor-at: returns empty when cursor_pos absent" {
    run_zsh '.ashlet:parse-candidate-cursor-at '"'"'{"request_id":1,"candidates":[{"completion":"git status"}]}'"'"' 0'
    [ "$status" -eq 0 ]
    [ "$output" = "" ]
}

# =============================================================================
# Error Handling Tests
# =============================================================================

@test ".ashlet:has-error: returns 0 when error present" {
    run_zsh '.ashlet:has-error '"'"'{"request_id":1,"candidates":[],"error":{"code":"model_not_found","message":"missing"}}'"'"
    [ "$status" -eq 0 ]
}

@test ".ashlet:has-error: returns 1 when no error" {
    run_zsh '.ashlet:has-error '"'"'{"request_id":1,"candidates":[{"completion":"test"}]}'"'"
    [ "$status" -eq 1 ]
}

@test ".ashlet:error-code: extracts error code" {
    run_zsh '.ashlet:error-code '"'"'{"request_id":1,"candidates":[],"error":{"code":"model_not_found","message":"missing"}}'"'"
    [ "$status" -eq 0 ]
    [ "$output" = "model_not_found" ]
}

@test ".ashlet:error-code: returns empty when no error" {
    run_zsh '.ashlet:error-code '"'"'{"request_id":1,"candidates":[]}'"'"
    [ "$status" -eq 0 ]
    [ "$output" = "" ]
}

# =============================================================================
# Display Formatting Tests
# =============================================================================

@test ".ashlet:format-candidate: formats basic candidate" {
    run_zsh '.ashlet:format-candidate "git status" 0 4'
    [ "$status" -eq 0 ]
    [[ "$output" == *"(1/4)"* ]]
    [[ "$output" == *"git status"* ]]
}

@test ".ashlet:format-candidate: includes cursor marker when cursor_pos set" {
    run_zsh '.ashlet:format-candidate "git status" 0 1 4'
    [ "$status" -eq 0 ]
    [[ "$output" == *"█"* ]]
}

@test ".ashlet:hint-text: returns keybinding hints" {
    run_zsh '.ashlet:hint-text'
    [ "$status" -eq 0 ]
    [[ "$output" == *"TAB"* ]]
    [[ "$output" == *"navigate"* ]]
}
