package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Generator performs text generation via an OpenAI-compatible API.
type Generator struct {
	baseURL     string
	apiKey      string
	model       string
	apiType     string // "responses" or "chat_completions"
	maxTokens   int
	temperature float64
	stop        []string
	telemetry   bool // send OpenRouter attribution headers
	client      *http.Client
}

// NewGenerator creates a generator from config.
func NewGenerator(baseURL, apiKey, model, apiType string, maxTokens int, temperature float64, stop []string, telemetry bool) *Generator {
	return &Generator{
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		apiType:     apiType,
		maxTokens:   maxTokens,
		temperature: temperature,
		stop:        stop,
		telemetry:   telemetry,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Generate sends a completion request to the API and returns the response text.
func (g *Generator) Generate(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	if g.apiType == "chat_completions" {
		return g.generateChatCompletions(ctx, systemPrompt, userMessage)
	}
	return g.generateResponses(ctx, systemPrompt, userMessage)
}

// Close is a no-op (no subprocess to manage).
func (g *Generator) Close() {}

// --- Responses API ---

type responsesRequest struct {
	Model       string           `json:"model"`
	Input       []responsesInput `json:"input"`
	MaxTokens   int              `json:"max_output_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stop        []string         `json:"stop,omitempty"`
}

type responsesInput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesResponse struct {
	Output []responsesOutput `json:"output"`
	Error  *apiError         `json:"error,omitempty"`
}

type responsesOutput struct {
	Type    string             `json:"type"`
	Content []responsesContent `json:"content,omitempty"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (g *Generator) generateResponses(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	reqBody := responsesRequest{
		Model: g.model,
		Input: []responsesInput{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   g.maxTokens,
		Temperature: g.temperature,
		Stop:        g.stop,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/responses", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	g.setHeaders(httpReq)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result responsesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	// Extract text from output
	for _, out := range result.Output {
		if out.Type == "message" {
			for _, c := range out.Content {
				if c.Type == "output_text" {
					return c.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no text content in response")
}

// --- Chat Completions API ---

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *apiError    `json:"error,omitempty"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

func (g *Generator) generateChatCompletions(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	reqBody := chatCompletionsRequest{
		Model: g.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   g.maxTokens,
		Temperature: g.temperature,
		Stop:        g.stop,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	g.setHeaders(httpReq)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result chatCompletionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// setHeaders sets common headers for API requests.
func (g *Generator) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}
	if g.telemetry {
		req.Header.Set("X-Title", "Ashlet - auto complete your shell commands")
		req.Header.Set("HTTP-Referer", "https://github.com/Paranoid-AF/ashlet")
	}
}
