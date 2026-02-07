#!/usr/bin/env zsh
# response.zsh - JSON response parsing for ashlet daemon responses

# Extract request_id from response
.ashlet:response-id() {
    local response="$1"
    print -r -- "$response" | jq -r '.request_id // empty'
}

# Count candidates in response
.ashlet:candidate-count() {
    local response="$1"
    print -r -- "$response" | jq -r '.candidates | length'
}

# Extract completion at index
.ashlet:parse-candidate-at() {
    local response="$1"
    local index="$2"
    print -r -- "$response" | jq -r ".candidates[$index].completion // empty"
}

# Extract cursor_pos at index (empty string if null/absent)
.ashlet:parse-candidate-cursor-at() {
    local response="$1"
    local index="$2"
    print -r -- "$response" | jq -r ".candidates[$index].cursor_pos // empty"
}

# Check if response contains an error (returns 0 if error present, 1 otherwise)
.ashlet:has-error() {
    local response="$1"
    local code
    code=$(print -r -- "$response" | jq -r '.error.code // empty')
    [[ -n "$code" ]]
}

# Extract error code from response
.ashlet:error-code() {
    local response="$1"
    print -r -- "$response" | jq -r '.error.code // empty'
}

# Extract error message from response
.ashlet:error-message() {
    local response="$1"
    print -r -- "$response" | jq -r '.error.message // empty'
}
