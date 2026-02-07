// Package defaults provides embedded default assets (prompt template and config).
package defaults

import _ "embed"

//go:embed default_prompt.md
var DefaultPrompt string

//go:embed default_config.json
var DefaultConfigJSON []byte
