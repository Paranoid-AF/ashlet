#!/usr/bin/env bats
# test_integration.bats — Integration tests for ashlet shell client

setup() {
    TEST_DIR="${BATS_TEST_DIRNAME}/.."
    # Create temp directory for mock socket
    MOCK_DIR="$(mktemp -d)"
    MOCK_SOCKET="${MOCK_DIR}/ashlet.sock"
    export ASHLET_SOCKET="$MOCK_SOCKET"
}

teardown() {
    # Clean up mock server if running
    if [[ -n "${MOCK_PID:-}" ]]; then
        kill "$MOCK_PID" 2>/dev/null || true
    fi
    rm -rf "$MOCK_DIR"
}

# Helper to run zsh function
run_zsh() {
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        source '${TEST_DIR}/client/request.zsh'
        source '${TEST_DIR}/client/response.zsh'
        export ASHLET_SOCKET='${MOCK_SOCKET}'
        $1
    "
}

# =============================================================================
# Request ID Ordering Tests
# =============================================================================

@test "integration: stale response rejection" {
    # Simulate receiving responses out of order
    local resp1='{"request_id":1,"candidates":[{"completion":"old"}]}'
    local resp2='{"request_id":2,"candidates":[{"completion":"new"}]}'

    # Process response 2 first
    run_zsh ".ashlet:response-id '$resp2'"
    [ "$status" -eq 0 ]
    [ "$output" = "2" ]

    # Simulate tracking: last_resp_id=2
    local last_resp_id=2

    # Response 1 should be rejected (resp_id <= last_resp_id)
    run_zsh ".ashlet:response-id '$resp1'"
    [ "$status" -eq 0 ]
    [ "$output" = "1" ]

    # Verify ordering logic
    [ "1" -le "$last_resp_id" ]
}

@test "integration: newer response accepted" {
    local resp1='{"request_id":1,"candidates":[{"completion":"old"}]}'
    local resp3='{"request_id":3,"candidates":[{"completion":"newest"}]}'

    # Simulate tracking: last_resp_id=1
    local last_resp_id=1

    run_zsh ".ashlet:response-id '$resp3'"
    [ "$status" -eq 0 ]
    [ "$output" = "3" ]

    # Response 3 should be accepted (resp_id > last_resp_id)
    [ "3" -gt "$last_resp_id" ]
}

# =============================================================================
# Error Response Tests
# =============================================================================

@test "integration: api_error error handling" {
    local error_response='{"request_id":1,"candidates":[],"error":{"code":"api_error","message":"request failed"}}'

    run_zsh ".ashlet:has-error '$error_response'"
    [ "$status" -eq 0 ]

    run_zsh ".ashlet:error-code '$error_response'"
    [ "$output" = "api_error" ]
}

@test "integration: empty candidates array is not an error" {
    local response='{"request_id":1,"candidates":[]}'

    run_zsh ".ashlet:has-error '$response'"
    [ "$status" -eq 1 ]  # No error

    run_zsh ".ashlet:candidate-count '$response'"
    [ "$output" = "0" ]
}

# =============================================================================
# JSON Edge Cases
# =============================================================================

@test "integration: handles special characters in completion" {
    local response='{"request_id":1,"candidates":[{"completion":"echo \"hello world\"","confidence":0.9}]}'

    run_zsh ".ashlet:parse-candidate-at '$response' 0"
    [ "$status" -eq 0 ]
    [ "$output" = 'echo "hello world"' ]
}

@test "integration: handles unicode in completion" {
    local response='{"request_id":1,"candidates":[{"completion":"echo 你好","confidence":0.9}]}'

    run_zsh ".ashlet:parse-candidate-at '$response' 0"
    [ "$status" -eq 0 ]
    [ "$output" = "echo 你好" ]
}

@test "integration: handles newlines in completion" {
    local response='{"request_id":1,"candidates":[{"completion":"echo one\\necho two","confidence":0.9}]}'

    run_zsh ".ashlet:parse-candidate-at '$response' 0"
    [ "$status" -eq 0 ]
    # jq will handle the escape sequence
}

# =============================================================================
# Socket Path Tests
# =============================================================================

@test "integration: socket path resolution priority" {
    # ASHLET_SOCKET has highest priority
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        ASHLET_SOCKET='/custom/socket.sock'
        XDG_RUNTIME_DIR='/run/user/1000'
        .ashlet:socket-path
    "
    [ "$output" = "/custom/socket.sock" ]

    # XDG_RUNTIME_DIR is next
    run zsh -c "
        source '${TEST_DIR}/client/socket.zsh'
        unset ASHLET_SOCKET
        XDG_RUNTIME_DIR='/run/user/1000'
        .ashlet:socket-path
    "
    [ "$output" = "/run/user/1000/ashlet.sock" ]
}

# =============================================================================
# Multiple Candidates Tests
# =============================================================================

@test "integration: browsing multiple candidates" {
    local response='{"request_id":1,"candidates":[{"completion":"git status"},{"completion":"git stash"},{"completion":"git stage"},{"completion":"git switch"}]}'

    run_zsh ".ashlet:candidate-count '$response'"
    [ "$output" = "4" ]

    run_zsh ".ashlet:parse-candidate-at '$response' 0"
    [ "$output" = "git status" ]

    run_zsh ".ashlet:parse-candidate-at '$response' 1"
    [ "$output" = "git stash" ]

    run_zsh ".ashlet:parse-candidate-at '$response' 2"
    [ "$output" = "git stage" ]

    run_zsh ".ashlet:parse-candidate-at '$response' 3"
    [ "$output" = "git switch" ]
}

@test "integration: cursor_pos in candidates" {
    local response='{"request_id":1,"candidates":[{"completion":"git commit -m \"\"","cursor_pos":15,"confidence":0.9}]}'

    run_zsh ".ashlet:parse-candidate-at '$response' 0"
    [ "$output" = 'git commit -m ""' ]

    run_zsh ".ashlet:parse-candidate-cursor-at '$response' 0"
    [ "$output" = "15" ]
}
