// Package protocol defines the request/response types for ashlet IPC.
// Messages are JSON-encoded and sent over a Unix domain socket, one per line.
package protocol

// Request is sent from the shell client to the daemon.
type Request struct {
	// Input is the current command line content.
	Input string `json:"input"`
	// CursorPos is the cursor position within the input.
	CursorPos int `json:"cursor_pos"`
	// Cwd is the current working directory of the shell.
	Cwd string `json:"cwd"`
	// SessionID identifies the shell session.
	SessionID string `json:"session_id"`
}

// Response is sent from the daemon back to the shell client.
type Response struct {
	// Completion is the suggested text to insert at the cursor position.
	Completion string `json:"completion"`
	// Confidence is the model's confidence score (0.0 to 1.0).
	Confidence float64 `json:"confidence"`
}
