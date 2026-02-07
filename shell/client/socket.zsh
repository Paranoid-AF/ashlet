#!/usr/bin/env zsh
# socket.zsh - Socket path resolution for ashlet daemon

# Resolve socket path: $ASHLET_SOCKET > $XDG_RUNTIME_DIR/ashlet.sock > /tmp/ashlet-$UID.sock
.ashlet:socket-path() {
    if [[ -n "${ASHLET_SOCKET:-}" ]]; then
        print -r -- "$ASHLET_SOCKET"
        return
    fi
    if [[ -n "${XDG_RUNTIME_DIR:-}" ]]; then
        print -r -- "${XDG_RUNTIME_DIR}/ashlet.sock"
        return
    fi
    print -r -- "/tmp/ashlet-${UID:-$(id -u)}.sock"
}

# Check if socket exists and is a socket file
.ashlet:socket-exists() {
    local socket_path
    socket_path="$(.ashlet:socket-path)"
    [[ -S "$socket_path" ]]
}
