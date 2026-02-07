# ashlet Shell Client Specification

This document specifies the behavior of the ashlet shell integration layer. It serves as a contract for refactoring and reimplementation.

## Overview

The shell client provides AI-powered command-line completion by:

1. Detecting input changes (buffer content OR cursor position) and requesting completions from the daemon
2. Displaying completion candidates below the prompt
3. Allowing the user to browse and apply candidates via keybindings

**Note:** This implementation is Zsh-only. Bash is not supported.

## Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           Shell Integration                                 │
│                                                                            │
│  ┌──────────────┐     ┌─────────────────────────────────────────────────┐ │
│  │ ashlet.zsh   │────▶│  autocomplete/init.zsh (entry point)            │ │
│  │ (entry point)│     │                                                 │ │
│  └──────────────┘     │  Sources:                                       │ │
│                       │  ├── client/socket.zsh   (socket resolution)    │ │
│                       │  ├── client/request.zsh  (IPC requests)         │ │
│                       │  ├── client/response.zsh (JSON parsing)         │ │
│                       │  ├── client/download.zsh (model download)       │ │
│                       │  ├── state.zsh           (state management)     │ │
│                       │  ├── display.zsh         (POSTDISPLAY)          │ │
│                       │  ├── async.zsh           (sysopen + zselect)    │ │
│                       │  ├── hooks.zsh           (ZLE hooks)            │ │
│                       │  ├── widgets.zsh         (user widgets)         │ │
│                       │  └── keybindings.zsh     (bindkey)              │ │
│                       └─────────────────────────────────────────────────┘ │
│                                                                            │
│  ┌──────────────┐                                                         │
│  │  _run.sh     │  (debug launcher)                                       │
│  └──────────────┘                                                         │
└────────────────────────────────────────────────────────────────────────────┘
                 │ Unix socket (JSON)
                 ▼
         ┌───────────────┐
         │    ashletd    │  (daemon)
         └───────────────┘
```

### Files

| Directory/File                 | Purpose                                               |
| ------------------------------ | ----------------------------------------------------- |
| `ashlet.zsh`                   | Entry point: sources `autocomplete/init.zsh`          |
| `autocomplete/init.zsh`        | Module loading, configuration, sources all components |
| `autocomplete/state.zsh`       | State variables and management functions              |
| `autocomplete/display.zsh`     | POSTDISPLAY + region_highlight rendering              |
| `autocomplete/async.zsh`       | Async I/O with sysopen, zselect, debounce             |
| `autocomplete/hooks.zsh`       | ZLE hook widgets (line-init, pre-redraw, finish)      |
| `autocomplete/widgets.zsh`     | User widgets (apply, navigate, dismiss, history)      |
| `autocomplete/keybindings.zsh` | Keybinding declarations                               |
| `client/socket.zsh`            | Socket path resolution                                |
| `client/request.zsh`           | IPC request building and sending                      |
| `client/response.zsh`          | JSON response parsing (jq-based)                      |
| `client/download.zsh`          | Model download logic                                  |
| `_run.sh`                      | Debug launcher for development/testing                |

### Design Principles

1. **Modular**: Each file has a single responsibility
2. **Zsh-native**: Uses zsh modules (zsh/system, zsh/zselect), add-zle-hook-widget
3. **Dot-prefix widgets**: Internal widgets use `.ashlet:*` naming to prevent plugin conflicts
4. **Debounced async**: Uses sysopen + zselect for non-blocking I/O with configurable delay
5. **Server-trusted candidates**: The client displays all candidates from the server, including typo corrections (e.g., `gti status` → `git status`). Request ID ordering handles staleness.

## IPC Protocol

### Transport

- Unix domain socket
- Path: `$ASHLET_SOCKET` > `$XDG_RUNTIME_DIR/ashlet.sock` > `/tmp/ashlet-$UID.sock`
- Tool: `socat` (required dependency)

### Request (JSON, single line)

```json
{
  "request_id": 42,
  "input": "git st",
  "cursor_pos": 6,
  "cwd": "/home/user/project",
  "session_id": "12345",
  "max_candidates": 4
}
```

| Field            | Type   | Description                             |
| ---------------- | ------ | --------------------------------------- |
| `request_id`     | int    | Monotonically increasing ID per session |
| `input`          | string | Current command line buffer             |
| `cursor_pos`     | int    | Cursor position (0-indexed byte offset) |
| `cwd`            | string | Current working directory               |
| `session_id`     | string | Shell PID (for session tracking)        |
| `max_candidates` | int    | Max completions to return (default: 4)  |

### Response (JSON, single line)

```json
{
  "request_id": 42,
  "candidates": [
    { "completion": "git status", "confidence": 0.95 },
    { "completion": "git stash", "confidence": 0.8, "cursor_pos": 10 }
  ]
}
```

| Field                     | Type    | Description                                      |
| ------------------------- | ------- | ------------------------------------------------ |
| `request_id`              | int     | Echoed from request (for ordering)               |
| `candidates`              | array   | Completion suggestions, highest confidence first |
| `candidates[].completion` | string  | Full command line (replaces entire buffer)       |
| `candidates[].confidence` | float   | Model confidence (0.0–1.0)                       |
| `candidates[].cursor_pos` | int?    | Cursor position after apply (null = end)         |
| `error`                   | object? | Error details if request failed                  |
| `error.code`              | string  | Machine-readable code (e.g., `model_not_found`)  |
| `error.message`           | string  | Human-readable description                       |
| `error.models`            | array?  | Model download info for `model_not_found`        |

## State Machine

```
                    ┌──────────────────────────────────────┐
                    │                                      │
                    ▼                                      │
┌─────────┐    ┌─────────┐    ┌───────────┐    ┌──────────┴─┐
│  IDLE   │───▶│FETCHING │───▶│  SHOWING  │───▶│  BROWSING  │
└─────────┘    └─────────┘    └───────────┘    └────────────┘
     ▲              │              │ │               │
     │              │          TAB │ │ ESC       TAB │
     │              ▼              ▼ ▼               │
     │         ┌─────────┐    ┌─────────┐           │
     └─────────│  ERROR  │    │DISMISSED│◀──────────┘
               └─────────┘    └─────────┘
```

### States

| State     | Description                                              |
| --------- | -------------------------------------------------------- |
| IDLE      | No candidates, no pending request                        |
| FETCHING  | Request in flight, may show stale candidates             |
| SHOWING   | Candidates displayed, browse_index = 0                   |
| BROWSING  | User navigating candidates with Shift+Arrow              |
| DISMISSED | User pressed ESC, candidates hidden until buffer changes |
| ERROR     | Daemon returned error (e.g., model missing)              |

### Trigger Conditions

**Candidates are re-fetched when:**

- Buffer content changes (typing, deletion)
- Cursor position changes (left/right arrow, mouse click, etc.)

This is critical because the Go server uses `cursor_pos` to determine context for completions.

**State comparison uses LBUFFER/RBUFFER:**

```zsh
.ashlet:same-state() {
    [[ $_ashlet_lbuffer == $LBUFFER && $_ashlet_rbuffer == $RBUFFER ]]
}
```

LBUFFER is text before cursor, RBUFFER is text after cursor. Together they capture both buffer content AND cursor position in a single comparison.

### History Navigation Guard

Candidates are only displayed when at the **latest history entry**. Up/Down arrows:

1. Pass through to shell history navigation
2. Clear displayed candidates
3. Resume candidate display when returning to latest entry

### Transitions

| From      | Event                                                    | To        | Action                                               |
| --------- | -------------------------------------------------------- | --------- | ---------------------------------------------------- |
| IDLE      | buffer OR cursor changes (len >= min_input, history tip) | FETCHING  | Trigger debounced async request                      |
| FETCHING  | response arrives                                         | SHOWING   | Store response, show candidate                       |
| FETCHING  | newer request sent                                       | FETCHING  | Discard old, track new                               |
| FETCHING  | buffer cleared                                           | IDLE      | Cancel request if possible                           |
| SHOWING   | buffer OR cursor changes                                 | FETCHING  | Invalidate candidates, send request                  |
| SHOWING   | TAB                                                      | IDLE      | Apply completion, clear state, allow re-fetch        |
| SHOWING   | Shift+Arrow                                              | BROWSING  | Update browse_index, redraw                          |
| SHOWING   | ESC                                                      | DISMISSED | Hide display, stop fetching                          |
| SHOWING   | Up/Down arrow                                            | IDLE      | Clear candidates, pass to shell history              |
| BROWSING  | TAB                                                      | IDLE      | Apply current candidate, clear state, allow re-fetch |
| BROWSING  | Shift+Arrow                                              | BROWSING  | Cycle browse_index                                   |
| BROWSING  | buffer changes                                           | FETCHING  | Invalidate, send request                             |
| BROWSING  | Up/Down arrow                                            | IDLE      | Clear candidates, pass to shell history              |
| DISMISSED | buffer changes (at history tip)                          | FETCHING  | Re-enable, send request                              |
| ANY       | line submitted (Enter)                                   | IDLE      | Full reset                                           |
| ANY       | new prompt                                               | IDLE      | Full reset                                           |

## Async Flow

```
Buffer OR Cursor change detected (in zle-line-pre-redraw)
        │
        ▼
.ashlet:trigger-async()
        │
        ├─► Cancel any pending wait_fd
        ├─► Save state (LBUFFER/RBUFFER)
        └─► Start debounce timer via sysopen + zselect
                │
                ▼ (after ASHLET_DELAY, default 50ms)
        .ashlet:wait-callback()
                │
                ├─► Check KEYS_QUEUED_COUNT/PENDING → abort if typing
                ├─► Check .ashlet:same-state → abort if changed
                └─► Call .ashlet:fetch-async()
                        │
                        ▼
                sysopen + socat to daemon (sends cursor_pos)
                        │
                        ▼ (response arrives)
                .ashlet:complete-callback()
                        │
                        ├─► Check .ashlet:same-state → abort if changed
                        ├─► Process response
                        ├─► Update POSTDISPLAY + region_highlight
                        └─► zle -R (force refresh)
```

## State Variables

| Variable                  | Type   | Description                                            |
| ------------------------- | ------ | ------------------------------------------------------ |
| `_ashlet_response`        | string | Raw JSON response (for parsing candidates)             |
| `_ashlet_candidate_count` | int    | Number of candidates in response                       |
| `_ashlet_browse_index`    | int    | Currently highlighted candidate (0-indexed)            |
| `_ashlet_dismissed`       | bool   | True if user pressed ESC                               |
| `_ashlet_at_history_tip`  | bool   | True if buffer is at the latest (newest) history entry |
| `_ashlet_lbuffer`         | string | Saved LBUFFER for state comparison                     |
| `_ashlet_rbuffer`         | string | Saved RBUFFER for state comparison                     |
| `_ashlet_next_req_id`     | int    | Counter for outgoing request IDs                       |
| `_ashlet_last_resp_id`    | int    | Highest response ID accepted                           |
| `_ashlet_wait_fd`         | fd     | File descriptor for debounce timer                     |
| `_ashlet_complete_fd`     | fd     | File descriptor for async response                     |

## Keybindings

| Key         | Sequence  | Widget                   | Action                                        |
| ----------- | --------- | ------------------------ | --------------------------------------------- |
| TAB         | `^I`      | `.ashlet:apply-tab`      | Apply current candidate or default completion |
| Shift+TAB   | `^[[Z`    | `expand-or-complete`     | Default shell completion                      |
| Shift+Left  | `^[[1;2D` | `.ashlet:prev-candidate` | Previous candidate (wrap)                     |
| Shift+Right | `^[[1;2C` | `.ashlet:next-candidate` | Next candidate (wrap)                         |
| ESC         | `^[`      | `.ashlet:dismiss`        | Dismiss candidates                            |
| Up          | `^[[A`    | `.ashlet:history-up`     | Shell history: previous command               |
| Down        | `^[[B`    | `.ashlet:history-down`   | Shell history: next command                   |

**Note:** ESC binding may conflict with vi-mode. Users can rebind if needed.

## Display Format

Candidates are shown on a line below the prompt:

```
(1/4) git status --short                    [↹] apply | [⇧]+[←/→] navigate
```

| Element         | Color         | Description                     |
| --------------- | ------------- | ------------------------------- | ------------------------------------------ |
| `(N/M)`         | Default       | Current index / total count     |
| Completion text | Default       | The candidate                   |
| `               | `             | Default                         | Cursor position marker (if cursor_pos set) |
| Hint            | Gray (fg=242) | Keybinding hints, right-aligned |

## ZLE Integration

### Hooks

Uses `add-zle-hook-widget` for proper hook chaining:

```zsh
add-zle-hook-widget line-init      .ashlet:line-init
add-zle-hook-widget line-pre-redraw .ashlet:line-pre-redraw
add-zle-hook-widget line-finish    .ashlet:line-finish
```

| Hook              | Purpose                                     |
| ----------------- | ------------------------------------------- |
| `line-init`       | Reset state on new prompt                   |
| `line-pre-redraw` | Detect buffer/cursor changes, trigger async |
| `line-finish`     | Clean up file descriptors, clear display    |

### Async I/O

Uses `sysopen` from zsh/system module for fd management:

```zsh
zmodload -F zsh/system b:sysopen
sysopen -r -o cloexec -u fd <(subprocess)
zle -Fw $fd callback
```

### Display

Uses `POSTDISPLAY` and `region_highlight`:

- POSTDISPLAY content starts at offset `${#BUFFER}`
- region_highlight for colors (gray hints)

## Configuration

| Variable                | Default | Description                    |
| ----------------------- | ------- | ------------------------------ |
| `ASHLET_SOCKET`         | (auto)  | Override socket path           |
| `ASHLET_MAX_CANDIDATES` | 4       | Max candidates to request      |
| `ASHLET_MIN_INPUT`      | 2       | Min chars before auto-fetching |
| `ASHLET_DELAY`          | 0.05    | Debounce delay in seconds      |

## Dependencies

| Tool     | Purpose           | Required      |
| -------- | ----------------- | ------------- |
| `zsh`    | Shell (5.3+)      | Yes           |
| `jq`     | JSON parsing      | Yes           |
| `socat`  | Unix socket IPC   | Yes           |
| `aria2c` | Parallel download | No (fallback) |
| `curl`   | HTTP download     | No (fallback) |
| `wget`   | HTTP download     | No (fallback) |

## Error Handling

| Error Code              | Behavior                                                    |
| ----------------------- | ----------------------------------------------------------- |
| `model_not_found`       | Prompt download on first occurrence, suppress after decline |
| `inference_unavailable` | Silent (llama-server not running)                           |
| Socket not found        | Silent fail (daemon not running)                            |
| Empty response          | Silent fail                                                 |
| JSON parse error        | Silent fail                                                 |

## Invariants

1. `_ashlet_response` and `_ashlet_candidate_count` MUST be updated atomically
2. `resp_id > last_resp_id` MUST hold for any accepted response
3. Display MUST reflect `_ashlet_response[browse_index]` when visible
4. TAB MUST apply exactly what was displayed
5. Buffer OR cursor change MUST trigger re-fetch (via debounce)
6. State comparison via LBUFFER/RBUFFER captures both buffer AND cursor

## Known Issues

1. **ESC conflicts** — ESC may conflict with vi-mode or other plugins
2. **Zsh 5.3+ required** — Uses `add-zle-hook-widget` which requires zsh 5.3+
