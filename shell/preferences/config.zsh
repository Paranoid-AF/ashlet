#!/usr/bin/env zsh
# config.zsh - Config file read/write operations for ashlet preferences

# Config file path resolution
.ashlet:config-dir() {
    if [[ -n "$ASHLET_CONFIG_DIR" ]]; then
        print -r -- "$ASHLET_CONFIG_DIR"
    elif [[ -n "$XDG_CONFIG_HOME" ]]; then
        print -r -- "$XDG_CONFIG_HOME/ashlet"
    else
        print -r -- "$HOME/.config/ashlet"
    fi
}

.ashlet:config-path() {
    print -r -- "$(.ashlet:config-dir)/config.json"
}

.ashlet:prompt-path() {
    print -r -- "$(.ashlet:config-dir)/prompt.md"
}

# Query daemon for canonical config (with defaults applied)
.ashlet:daemon-config() {
    local socket_path="$(.ashlet:socket-path)"
    [[ -S "$socket_path" ]] || return 1
    local response
    response=$(print -r -- '{"action":"get"}' | socat -t2 - "UNIX-CONNECT:$socket_path" 2>/dev/null) || return 1
    # Extract .config from ConfigResponse
    print -r -- "$response" | command jq -e '.config // empty' 2>/dev/null
}

# Query daemon for embedded default config
.ashlet:daemon-defaults() {
    local socket_path="$(.ashlet:socket-path)"
    [[ -S "$socket_path" ]] || return 1
    local response
    response=$(print -r -- '{"action":"defaults"}' | socat -t2 - "UNIX-CONNECT:$socket_path" 2>/dev/null) || return 1
    print -r -- "$response" | command jq -e '.config // empty' 2>/dev/null
}

# Query daemon for default prompt
.ashlet:daemon-prompt() {
    local socket_path="$(.ashlet:socket-path)"
    [[ -S "$socket_path" ]] || return 1
    local response
    response=$(print -r -- '{"action":"default_prompt"}' | socat -t2 - "UNIX-CONNECT:$socket_path" 2>/dev/null) || return 1
    print -r -- "$response" | command jq -re '.prompt // empty' 2>/dev/null
}

# Load config: daemon first, file fallback
.ashlet:load-config() {
    # Try daemon first
    local daemon_cfg
    daemon_cfg="$(.ashlet:daemon-config)" && [[ -n "$daemon_cfg" ]] && {
        print -r -- "$daemon_cfg"
        return 0
    }
    # Fallback: read file directly
    local config_path="$(.ashlet:config-path)"
    if [[ -f "$config_path" ]]; then
        print -r -- "$(<"$config_path")"
    else
        print -r -- '{}'
    fi
}

# Save config JSON to file
.ashlet:save-config() {
    local config_json="$1"
    local config_dir="$(.ashlet:config-dir)"
    local config_path="$(.ashlet:config-path)"

    mkdir -p "$config_dir" || {
        print "ashlet: failed to create config directory: $config_dir" >&2
        return 1
    }

    print -r -- "$config_json" > "$config_path" || {
        print "ashlet: failed to write config file: $config_path" >&2
        return 1
    }

    return 0
}

# Get a config value using jq
.ashlet:config-get() {
    local query="$1"
    .ashlet:load-config | command jq -r "$query // empty"
}
