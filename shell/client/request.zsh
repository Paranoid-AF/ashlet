#!/usr/bin/env zsh
# request.zsh - IPC request building and sending for ashlet daemon

# Send request to daemon and return response
# Usage: .ashlet:request <request_id> <input> <cursor_pos> <cwd> <session_id> [max_candidates]
.ashlet:request() {
    local request_id="$1"
    local input="$2"
    local cursor_pos="$3"
    local cwd="$4"
    local session_id="$5"
    local max_candidates="${6:-$ASHLET_MAX_CANDIDATES}"
    local socket_path
    socket_path="$(.ashlet:socket-path)"

    # Check if socket exists
    if [[ ! -S "$socket_path" ]]; then
        return 1
    fi

    # Build JSON request - escape special characters in input
    local json_input json_cwd
    json_input=$(print -r -- "$input" | jq -Rs '.')
    json_cwd=$(print -r -- "$cwd" | jq -Rs '.')

    local request
    request=$(printf '{"request_id":%d,"input":%s,"cursor_pos":%d,"cwd":%s,"session_id":"%s","max_candidates":%d}' \
        "$request_id" "$json_input" "$cursor_pos" "$json_cwd" "$session_id" "$max_candidates")

    # Send request and get response.
    # -t10: wait up to 10s for the server response after sending the request.
    # Inference with dir context can take 3-5s; socat exits immediately once
    # the server closes the connection, so this only affects the worst case.
    print -r -- "$request" | socat -t10 - "UNIX-CONNECT:$socket_path" 2>/dev/null
}

# Send a context warm-up request (fire-and-forget)
# Usage: .ashlet:context-request <cwd>
.ashlet:context-request() {
    local cwd="$1"
    local socket_path
    socket_path="$(.ashlet:socket-path)"

    # Check if socket exists
    if [[ ! -S "$socket_path" ]]; then
        return 1
    fi

    # JSON-escape cwd using zsh builtins (avoid jq for this simple case)
    local escaped="${cwd//\\/\\\\}"
    escaped="${escaped//\"/\\\"}"

    local request="{\"type\":\"context\",\"cwd\":\"${escaped}\"}"

    # Fire-and-forget in background
    (print -r -- "$request" | socat -t1 - "UNIX-CONNECT:$socket_path" &>/dev/null &)
}
