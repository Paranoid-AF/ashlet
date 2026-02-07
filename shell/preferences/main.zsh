#!/usr/bin/env zsh
# main.zsh - ashlet preferences: open config/prompt in $EDITOR, reload daemon

# Source config read/write helpers
local basedir="${0:A:h}"
source "${basedir}/config.zsh" || return 1

# Create default config.json if it doesn't exist
.ashlet:ensure-config() {
    emulate -L zsh
    local config_path="$(.ashlet:config-path)"
    if [[ -f "$config_path" ]]; then
        return 0
    fi

    # Only create the file if the daemon can provide the full config with defaults
    local config_json
    config_json="$(.ashlet:daemon-config)" && [[ -n "$config_json" ]] || {
        # Daemon unavailable — try defaults action
        config_json="$(.ashlet:daemon-defaults)" && [[ -n "$config_json" ]] || return 1
    }
    .ashlet:save-config "$config_json"
}

# Create default prompt.md if it doesn't exist
.ashlet:ensure-prompt() {
    emulate -L zsh
    local prompt_path="$(.ashlet:prompt-path)"
    if [[ -f "$prompt_path" ]]; then
        return 0
    fi

    local prompt_text
    prompt_text="$(.ashlet:daemon-prompt)" && [[ -n "$prompt_text" ]] || return 1

    local config_dir="$(.ashlet:config-dir)"
    mkdir -p "$config_dir" || {
        print "ashlet: failed to create config directory: $config_dir" >&2
        return 1
    }

    print -r -- "$prompt_text" > "$prompt_path" || {
        print "ashlet: failed to write prompt file: $prompt_path" >&2
        return 1
    }
}

# Reload daemon config over the socket.
.ashlet:reload-daemon() {
    emulate -L zsh
    local socket_path="$(.ashlet:socket-path)"

    if [[ ! -S "$socket_path" ]]; then
        print "ashlet: daemon not running, changes will apply on next start" >&2
        return 0
    fi

    local response
    response=$(print -r -- '{"action":"reload"}' | socat -t5 - "UNIX-CONNECT:$socket_path" 2>/dev/null)

    if [[ -n "$response" ]]; then
        print "ashlet: daemon reloaded" >&2
        return 0
    else
        print "ashlet: failed to reload daemon" >&2
        return 1
    fi
}

# Reset config and prompt to defaults
.ashlet:reset-config() {
    emulate -L zsh
    local config_path="$(.ashlet:config-path)"
    local prompt_path="$(.ashlet:prompt-path)"

    if [[ -f "$config_path" || -f "$prompt_path" ]]; then
        print -n "ashlet: reset config and prompt to defaults? [y/N] " >&2
        local answer
        read -r answer </dev/tty
        [[ "$answer" == [yY] ]] || { print "ashlet: cancelled" >&2; return 1; }
    fi

    if [[ -f "$config_path" ]]; then
        rm -f "$config_path"
        print "ashlet: config file removed" >&2
    fi
    if [[ -f "$prompt_path" ]]; then
        rm -f "$prompt_path"
        print "ashlet: prompt file removed" >&2
    fi

    print "ashlet: defaults restored (embedded defaults will be used)" >&2
    .ashlet:reload-daemon
}

# Print usage
.ashlet:usage() {
    emulate -L zsh
    print "usage: ashlet [--config | --prompt | --reset | --help]" >&2
    print "  (no args)    ask to edit config or prompt" >&2
    print "  --config/-c  open config.json in \$EDITOR" >&2
    print "  --prompt/-p  open prompt.md in \$EDITOR" >&2
    print "  --reset      restore default configuration" >&2
    print "  --help/-h    show this help" >&2
}

# Edit config in $EDITOR
.ashlet:edit-config() {
    emulate -L zsh
    .ashlet:ensure-config || {
        print "ashlet: failed to create config file" >&2
        return 1
    }

    local config_path="$(.ashlet:config-path)"
    local editor="${EDITOR:-${VISUAL:-vi}}"

    "$editor" "$config_path"
    .ashlet:reload-daemon
}

# Edit prompt in $EDITOR
.ashlet:edit-prompt() {
    emulate -L zsh
    .ashlet:ensure-prompt || {
        print "ashlet: failed to create prompt file" >&2
        return 1
    }

    local prompt_path="$(.ashlet:prompt-path)"
    local editor="${EDITOR:-${VISUAL:-vi}}"

    "$editor" "$prompt_path"
    .ashlet:reload-daemon
}

# Main entry point - the 'ashlet' command
ashlet() {
    emulate -L zsh
    # Restore stdout/stderr if redirected (e.g. by async fd handling)
    [[ -t 1 ]] || exec 1>/dev/tty
    [[ -t 2 ]] || exec 2>/dev/tty
    case "$1" in
        --config|-c)
            .ashlet:edit-config
            ;;
        --prompt|-p)
            .ashlet:edit-prompt
            ;;
        --reset)
            .ashlet:reset-config
            ;;
        --help|-h)
            .ashlet:usage
            ;;
        "")
            print -n "ashlet: edit (c)onfig or (p)rompt? [c/p] " >&2
            local answer
            read -r answer </dev/tty
            case "$answer" in
                c|C)
                    .ashlet:edit-config
                    ;;
                p|P)
                    .ashlet:edit-prompt
                    ;;
                *)
                    print "ashlet: cancelled" >&2
                    return 1
                    ;;
            esac
            ;;
        *)
            print "ashlet: unknown option: $1" >&2
            .ashlet:usage
            return 1
            ;;
    esac
}
