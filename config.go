package ashlet

import (
	"encoding/json"
	"os"
	"path/filepath"

	defaults "github.com/Paranoid-AF/ashlet/default"
)

// Config represents the user's ashlet configuration.
type Config struct {
	Version    int              `json:"version"`
	Generation GenerationConfig `json:"generation"`
	Embedding  EmbeddingConfig  `json:"embedding"`
	Telemetry  TelemetryConfig  `json:"telemetry"`
}

// GenerationConfig holds settings for the generation API.
type GenerationConfig struct {
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	APIType      string   `json:"api_type"`
	Model        string   `json:"model"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	Temperature  float64  `json:"temperature,omitempty"`
	Stop         []string `json:"stop,omitempty"`
	NoRawHistory *bool    `json:"no_raw_history,omitempty"`
}

// EmbeddingConfig holds settings for the embedding API.
type EmbeddingConfig struct {
	BaseURL            string `json:"base_url"`
	APIKey             string `json:"api_key"`
	Model              string `json:"model"`
	Dimensions         int    `json:"dimensions,omitempty"`
	TTLMinutes         int    `json:"ttl_minutes,omitempty"`
	MaxHistoryCommands int    `json:"max_history_commands,omitempty"`
}

// TelemetryConfig holds telemetry settings.
type TelemetryConfig struct {
	OpenRouter *bool `json:"openrouter,omitempty"`
}

// ConfigDir returns the config directory path.
// Resolution order: $ASHLET_CONFIG_DIR > $XDG_CONFIG_HOME/ashlet > ~/.config/ashlet
func ConfigDir() string {
	if dir := os.Getenv("ASHLET_CONFIG_DIR"); dir != "" {
		return dir
	}
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "ashlet")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", "ashlet-config")
	}
	return filepath.Join(home, ".config", "ashlet")
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// PromptPath returns the prompt file path.
func PromptPath() string {
	return filepath.Join(ConfigDir(), "prompt.md")
}

// DefaultConfig returns the default configuration from the embedded default_config.json.
func DefaultConfig() *Config {
	var cfg Config
	if err := json.Unmarshal(defaults.DefaultConfigJSON, &cfg); err != nil {
		panic("ashlet: invalid embedded default_config.json: " + err.Error())
	}
	return &cfg
}

// LoadConfig loads config from disk or returns defaults if not found.
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults for missing fields
	defaults := DefaultConfig()
	if cfg.Generation.BaseURL == "" {
		cfg.Generation.BaseURL = defaults.Generation.BaseURL
	}
	if cfg.Generation.APIType == "" {
		cfg.Generation.APIType = defaults.Generation.APIType
	}
	if cfg.Generation.Model == "" {
		cfg.Generation.Model = defaults.Generation.Model
	}
	if cfg.Generation.MaxTokens == 0 {
		cfg.Generation.MaxTokens = defaults.Generation.MaxTokens
	}
	if cfg.Generation.Temperature == 0 {
		cfg.Generation.Temperature = defaults.Generation.Temperature
	}
	if cfg.Embedding.Model == "" {
		cfg.Embedding.Model = defaults.Embedding.Model
	}
	if cfg.Embedding.Dimensions == 0 {
		cfg.Embedding.Dimensions = defaults.Embedding.Dimensions
	}
	if cfg.Embedding.TTLMinutes == 0 {
		cfg.Embedding.TTLMinutes = defaults.Embedding.TTLMinutes
	}
	if cfg.Embedding.MaxHistoryCommands == 0 {
		cfg.Embedding.MaxHistoryCommands = defaults.Embedding.MaxHistoryCommands
	}
	if cfg.Generation.NoRawHistory == nil {
		cfg.Generation.NoRawHistory = defaults.Generation.NoRawHistory
	}
	if cfg.Telemetry.OpenRouter == nil {
		cfg.Telemetry.OpenRouter = defaults.Telemetry.OpenRouter
	}

	return &cfg, nil
}

// ValidateConfig checks configuration for potential issues and returns warnings.
func ValidateConfig(cfg *Config) []string {
	var warnings []string
	if cfg == nil {
		return warnings
	}
	if cfg.Generation.NoRawHistory != nil && *cfg.Generation.NoRawHistory && !EmbeddingEnabled(cfg) {
		warnings = append(warnings, "no_raw_history is enabled but embedding API key is not configured; history context will be unavailable")
	}
	return warnings
}

// ResolveGenerationBaseURL returns the generation API base URL.
// Priority: $ASHLET_GENERATION_API_BASE_URL env > config value.
func ResolveGenerationBaseURL(cfg *Config) string {
	if url := os.Getenv("ASHLET_GENERATION_API_BASE_URL"); url != "" {
		return url
	}
	if cfg != nil {
		return cfg.Generation.BaseURL
	}
	return ""
}

// ResolveGenerationAPIKey returns the generation API key.
// Priority: $ASHLET_GENERATION_API_KEY env > config value.
func ResolveGenerationAPIKey(cfg *Config) string {
	if key := os.Getenv("ASHLET_GENERATION_API_KEY"); key != "" {
		return key
	}
	if cfg != nil {
		return cfg.Generation.APIKey
	}
	return ""
}

// ResolveGenerationModel returns the generation model name.
// Priority: $ASHLET_GENERATION_MODEL env > config value.
func ResolveGenerationModel(cfg *Config) string {
	if model := os.Getenv("ASHLET_GENERATION_MODEL"); model != "" {
		return model
	}
	if cfg != nil {
		return cfg.Generation.Model
	}
	return ""
}

// ResolveEmbeddingBaseURL returns the embedding API base URL.
// Priority: $ASHLET_EMBEDDING_API_BASE_URL env > config value.
func ResolveEmbeddingBaseURL(cfg *Config) string {
	if url := os.Getenv("ASHLET_EMBEDDING_API_BASE_URL"); url != "" {
		return url
	}
	if cfg != nil {
		return cfg.Embedding.BaseURL
	}
	return ""
}

// ResolveEmbeddingAPIKey returns the embedding API key.
// Priority: $ASHLET_EMBEDDING_API_KEY env > config value.
func ResolveEmbeddingAPIKey(cfg *Config) string {
	if key := os.Getenv("ASHLET_EMBEDDING_API_KEY"); key != "" {
		return key
	}
	if cfg != nil {
		return cfg.Embedding.APIKey
	}
	return ""
}

// ResolveEmbeddingModel returns the embedding model name.
// Priority: $ASHLET_EMBEDDING_MODEL env > config value.
func ResolveEmbeddingModel(cfg *Config) string {
	if model := os.Getenv("ASHLET_EMBEDDING_MODEL"); model != "" {
		return model
	}
	if cfg != nil {
		return cfg.Embedding.Model
	}
	return ""
}

// EmbeddingEnabled returns true when both base_url and api_key are configured for embedding.
func EmbeddingEnabled(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	return ResolveEmbeddingBaseURL(cfg) != "" && ResolveEmbeddingAPIKey(cfg) != ""
}

// OpenRouterTelemetryEnabled returns whether OpenRouter attribution headers should be sent.
func OpenRouterTelemetryEnabled(cfg *Config) bool {
	if cfg == nil || cfg.Telemetry.OpenRouter == nil {
		return true // default true
	}
	return *cfg.Telemetry.OpenRouter
}
