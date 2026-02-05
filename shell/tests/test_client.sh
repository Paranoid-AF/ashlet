#!/usr/bin/env bats
# ashlet — Shell client tests

setup() {
  ASHLET_SCRIPT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)"
  source "${ASHLET_SCRIPT_DIR}/client.sh"
}

@test "socket path uses ASHLET_SOCKET when set" {
  ASHLET_SOCKET="/custom/path.sock"
  result="$(_ashlet_socket_path)"
  [ "$result" = "/custom/path.sock" ]
}

@test "socket path falls back to XDG_RUNTIME_DIR" {
  unset ASHLET_SOCKET
  export XDG_RUNTIME_DIR="/run/user/1000"
  result="$(_ashlet_socket_path)"
  [ "$result" = "/run/user/1000/ashlet.sock" ]
}

@test "socket path falls back to /tmp" {
  unset ASHLET_SOCKET
  unset XDG_RUNTIME_DIR
  result="$(_ashlet_socket_path)"
  [[ "$result" == /tmp/ashlet-*.sock ]]
}

@test "parse completion extracts value with jq" {
  if ! command -v jq >/dev/null 2>&1; then
    skip "jq not installed"
  fi
  result="$(_ashlet_parse_completion '{"completion":"git status","confidence":0.9}')"
  [ "$result" = "git status" ]
}

@test "request fails when socket does not exist" {
  export ASHLET_SOCKET="/nonexistent/path.sock"
  run _ashlet_request "git st" 6 "/home/user" "12345"
  [ "$status" -ne 0 ]
}
