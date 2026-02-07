// Package ashlet defines the request/response types for ashlet IPC.
// Messages are JSON-encoded and sent over a Unix domain socket, one per line.
package ashlet

// Request is sent from the shell client to the daemon.
type Request struct {
	// RequestID is a per-session incrementing identifier assigned by the shell.
	// The daemon echoes it back in the response for ordering.
	RequestID int `json:"request_id"`
	// Input is the current command line content.
	Input string `json:"input"`
	// CursorPos is the cursor position within the input.
	CursorPos int `json:"cursor_pos"`
	// Cwd is the current working directory of the shell.
	Cwd string `json:"cwd"`
	// SessionID identifies the shell session.
	SessionID string `json:"session_id"`
	// MaxCandidates is the maximum number of completion candidates to return.
	MaxCandidates int `json:"max_candidates,omitempty"`
}

// Candidate represents a single completion suggestion with a confidence score.
type Candidate struct {
	// Completion is the full command line suggestion.
	Completion string `json:"completion"`
	// CursorPos is the desired cursor position within the completion.
	// nil means cursor at end of completion.
	CursorPos *int `json:"cursor_pos,omitempty"`
	// Confidence is the model's confidence score (0.0 to 1.0).
	Confidence float64 `json:"confidence"`
}

// Response is sent from the daemon back to the shell client.
type Response struct {
	// RequestID is echoed from the request for ordering on the client side.
	RequestID int `json:"request_id"`
	// Candidates is the list of completion suggestions, sorted by confidence descending.
	Candidates []Candidate `json:"candidates"`
	// Error is set when the daemon cannot fulfill the request.
	Error *Error `json:"error,omitempty"`
}

// Error describes a daemon-side error returned to the shell client.
type Error struct {
	// Code is a machine-readable error identifier (e.g. "not_configured", "api_error").
	Code string `json:"code"`
	// Message is a human-readable error description.
	Message string `json:"message"`
}

// ContextRequest is sent from the shell client to warm the directory context cache.
type ContextRequest struct {
	// Type is always "context".
	Type string `json:"type"`
	// Cwd is the directory to pre-cache context for.
	Cwd string `json:"cwd"`
}

// ContextResponse is sent from the daemon in response to a ContextRequest.
type ContextResponse struct {
	// OK is true when the warm-up was accepted.
	OK bool `json:"ok"`
	// Error is set when the operation fails.
	Error *Error `json:"error,omitempty"`
}

// ConfigRequest is sent from the shell client for configuration operations.
type ConfigRequest struct {
	// Action is the config operation: "get", "reload", "defaults", or "default_prompt".
	Action string `json:"action"`
}

// ConfigResponse is sent from the daemon in response to a ConfigRequest.
type ConfigResponse struct {
	// Config is the current configuration (for "get", "reload", and "defaults" actions).
	Config *Config `json:"config,omitempty"`
	// Prompt is the default prompt template (for "default_prompt" action).
	Prompt string `json:"prompt,omitempty"`
	// Warnings contains configuration warnings (for "validate" action).
	Warnings []string `json:"warnings,omitempty"`
	// Error is set when the operation fails.
	Error *Error `json:"error,omitempty"`
}
