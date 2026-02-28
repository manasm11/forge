package provider

import (
	"fmt"
	"strings"
	"time"
)

// ProviderType identifies the model backend.
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOllama    ProviderType = "ollama"
)

// Config holds the user's provider selection. Persisted in state.Settings.
type Config struct {
	Type      ProviderType `json:"type"`
	Model     string       `json:"model"`
	OllamaURL string      `json:"ollama_url,omitempty"`
}

// OllamaStatus represents the result of a DetectOllama call.
type OllamaStatus struct {
	Available bool
	URL       string
	Version   string        // Ollama server version if available
	Models    []OllamaModel // populated only if Available is true
	Error     string        // non-empty if detection failed
	Latency   time.Duration // round-trip time of health check
}

// OllamaModel represents a model available in the local Ollama instance.
type OllamaModel struct {
	Name       string    // e.g. "qwen3-coder:latest"
	Size       int64     // model size in bytes
	Family     string    // model family e.g. "qwen3"
	ModifiedAt time.Time // last modified
}

// DefaultOllamaURL returns the standard local Ollama endpoint.
func DefaultOllamaURL() string {
	return "http://localhost:11434"
}

// DefaultConfig returns the default provider config (Anthropic + sonnet).
func DefaultConfig() Config {
	return Config{
		Type:  ProviderAnthropic,
		Model: "sonnet",
	}
}

// EnvVarsForProvider returns the environment variables the claude CLI needs
// to connect to the selected provider.
//   - Anthropic: empty map (claude uses its default behavior).
//   - Ollama: ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_API_KEY.
func EnvVarsForProvider(cfg Config) map[string]string {
	if cfg.Type == ProviderAnthropic {
		return map[string]string{}
	}

	url := cfg.OllamaURL
	if url == "" {
		url = DefaultOllamaURL()
	}

	return map[string]string{
		"ANTHROPIC_BASE_URL":   url,
		"ANTHROPIC_AUTH_TOKEN": "ollama",
		"ANTHROPIC_API_KEY":    "ollama",
	}
}

// ValidateConfig checks that a provider config is valid.
// Returns a slice of error messages (empty = valid).
func ValidateConfig(cfg Config) []string {
	var errs []string

	if cfg.Type == "" {
		errs = append(errs, "provider type is required")
	} else if cfg.Type != ProviderAnthropic && cfg.Type != ProviderOllama {
		errs = append(errs, fmt.Sprintf("unknown provider type: %q", cfg.Type))
	}

	if cfg.Model == "" {
		errs = append(errs, "model is required")
	}

	if cfg.Type == ProviderOllama && cfg.OllamaURL != "" {
		if !strings.HasPrefix(cfg.OllamaURL, "http://") && !strings.HasPrefix(cfg.OllamaURL, "https://") {
			errs = append(errs, fmt.Sprintf("invalid Ollama URL: %q (must start with http:// or https://)", cfg.OllamaURL))
		}
	}

	return errs
}

// FormatModelName shortens display names (strips ":latest" suffix).
func FormatModelName(name string) string {
	return strings.TrimSuffix(name, ":latest")
}

// FormatModelSize returns a human-readable model size like "7.6 GB".
func FormatModelSize(bytes int64) string {
	switch {
	case bytes >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB", float64(bytes)/1_000_000_000)
	case bytes >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1_000_000)
	case bytes >= 1_000:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1_000)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// RecommendedModels returns model names known to work well with Claude Code
// for each provider type. Used as hints in the UI, not as a restriction.
func RecommendedModels(pt ProviderType) []string {
	if pt == ProviderAnthropic {
		return []string{"sonnet", "opus", "haiku"}
	}
	return []string{"qwen3-coder", "glm-4.7-flash", "gpt-oss:20b", "devstral-small"}
}

// ModelInList checks if a model name exists in a list of OllamaModels.
// Matches both full name ("qwen3-coder:latest") and short name ("qwen3-coder").
func ModelInList(name string, models []OllamaModel) bool {
	if name == "" {
		return false
	}
	for _, m := range models {
		if m.Name == name || FormatModelName(m.Name) == name {
			return true
		}
		// Also match if the input has a tag against the short form
		if FormatModelName(name) == FormatModelName(m.Name) {
			return true
		}
		// Match short name against model with tag (e.g., "gpt-oss" against "gpt-oss:20b")
		if strings.HasPrefix(m.Name, name+":") {
			return true
		}
	}
	return false
}

// MergeEnvVars merges provider env vars into an existing env var map.
// Provider vars take precedence (overwrite) on key collision.
func MergeEnvVars(existing, providerVars map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range existing {
		result[k] = v
	}
	for k, v := range providerVars {
		result[k] = v
	}
	return result
}