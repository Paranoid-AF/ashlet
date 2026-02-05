// Package embedding provides an interface for generating embeddings via llama.cpp.
package embedding

// Embedder generates vector embeddings from text input.
type Embedder struct {
	modelPath string
}

// NewEmbedder creates an embedder using the given GGUF model file.
func NewEmbedder(modelPath string) *Embedder {
	return &Embedder{modelPath: modelPath}
}

// Embed generates an embedding vector for the given text.
func (e *Embedder) Embed(text string) ([]float32, error) {
	// TODO: invoke llama.cpp embedding binary or use cgo bindings
	return nil, nil
}
