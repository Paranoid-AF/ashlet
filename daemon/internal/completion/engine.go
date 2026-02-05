// Package completion orchestrates model inference to generate shell completions.
package completion

import (
	"github.com/Paranoid-AF/ashlet/internal/context"
	"github.com/Paranoid-AF/ashlet/pkg/protocol"
)

// Engine orchestrates context gathering and model inference for completions.
type Engine struct {
	gatherer *context.Gatherer
}

// NewEngine creates a new completion engine.
func NewEngine() *Engine {
	return &Engine{
		gatherer: context.NewGatherer(),
	}
}

// Complete processes a completion request and returns a response.
func (e *Engine) Complete(req *protocol.Request) *protocol.Response {
	ctx := e.gatherer.Gather(req)

	// TODO: pass context to model inference layer for actual completion
	_ = ctx

	return &protocol.Response{
		Completion: "",
		Confidence: 0.0,
	}
}
