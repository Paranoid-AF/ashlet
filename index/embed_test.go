package index

import (
	"testing"
)

func TestEmbedderCreation(t *testing.T) {
	e := NewEmbedder("http://localhost:8080", "test-key", "test-model")
	if e.baseURL != "http://localhost:8080" {
		t.Errorf("expected baseURL http://localhost:8080, got %s", e.baseURL)
	}
	if e.apiKey != "test-key" {
		t.Errorf("expected apiKey test-key, got %s", e.apiKey)
	}
	if e.model != "test-model" {
		t.Errorf("expected model test-model, got %s", e.model)
	}
}

func TestEmbedBatchEmpty(t *testing.T) {
	e := NewEmbedder("http://localhost:8080", "test-key", "test-model")
	result, err := e.EmbedBatch(nil)
	if err != nil {
		t.Fatalf("unexpected error for empty batch: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty batch, got %v", result)
	}
}
