package provider

import (
	"testing"
)

// ============================================================
// EnvVarsForProvider
// ============================================================

func TestEnvVarsForProvider_Anthropic(t *testing.T) {
	t.Parallel()
	cfg := Config{Type: ProviderAnthropic, Model: "sonnet"}
	env := EnvVarsForProvider(cfg)

	if len(env) != 0 {
		t.Errorf("Anthropic should return empty env vars, got %v", env)
	}
}

func TestEnvVarsForProvider_Ollama_Default(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Type:      ProviderOllama,
		Model:     "qwen3-coder",
		OllamaURL: "http://localhost:11434",
	}
	env := EnvVarsForProvider(cfg)

	if env["ANTHROPIC_BASE_URL"] != "http://localhost:11434" {
		t.Errorf("ANTHROPIC_BASE_URL = %q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "ollama" {
		t.Errorf("ANTHROPIC_AUTH_TOKEN = %q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["ANTHROPIC_API_KEY"] != "" {
		t.Errorf("ANTHROPIC_API_KEY should be empty string, got %q", env["ANTHROPIC_API_KEY"])
	}
}

func TestEnvVarsForProvider_Ollama_CustomURL(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Type:      ProviderOllama,
		Model:     "glm-4.7-flash",
		OllamaURL: "http://192.168.1.100:11434",
	}
	env := EnvVarsForProvider(cfg)

	if env["ANTHROPIC_BASE_URL"] != "http://192.168.1.100:11434" {
		t.Errorf("ANTHROPIC_BASE_URL = %q", env["ANTHROPIC_BASE_URL"])
	}
}

func TestEnvVarsForProvider_Ollama_EmptyURL_UsesDefault(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Type:      ProviderOllama,
		Model:     "qwen3-coder",
		OllamaURL: "", // empty
	}
	env := EnvVarsForProvider(cfg)

	if env["ANTHROPIC_BASE_URL"] != DefaultOllamaURL() {
		t.Errorf("should fall back to default URL, got %q", env["ANTHROPIC_BASE_URL"])
	}
}

// ============================================================
// ValidateConfig
// ============================================================

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cfg       Config
		wantValid bool
	}{
		{
			name:      "valid anthropic",
			cfg:       Config{Type: ProviderAnthropic, Model: "sonnet"},
			wantValid: true,
		},
		{
			name:      "valid ollama",
			cfg:       Config{Type: ProviderOllama, Model: "qwen3-coder", OllamaURL: "http://localhost:11434"},
			wantValid: true,
		},
		{
			name:      "ollama without url still valid (uses default)",
			cfg:       Config{Type: ProviderOllama, Model: "qwen3-coder"},
			wantValid: true,
		},
		{
			name:      "empty model",
			cfg:       Config{Type: ProviderAnthropic, Model: ""},
			wantValid: false,
		},
		{
			name:      "empty type",
			cfg:       Config{Type: "", Model: "sonnet"},
			wantValid: false,
		},
		{
			name:      "invalid type",
			cfg:       Config{Type: "openai", Model: "gpt-4"},
			wantValid: false,
		},
		{
			name:      "ollama with invalid url",
			cfg:       Config{Type: ProviderOllama, Model: "qwen3-coder", OllamaURL: "not-a-url"},
			wantValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateConfig(tt.cfg)
			if tt.wantValid && len(errs) > 0 {
				t.Errorf("expected valid, got errors: %v", errs)
			}
			if !tt.wantValid && len(errs) == 0 {
				t.Error("expected errors, got valid")
			}
		})
	}
}

// ============================================================
// FormatModelName
// ============================================================

func TestFormatModelName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"qwen3-coder:latest", "qwen3-coder"},
		{"glm-4.7-flash:latest", "glm-4.7-flash"},
		{"gpt-oss:20b", "gpt-oss:20b"},            // non-latest tag preserved
		{"qwen3-coder", "qwen3-coder"},              // no tag at all
		{"deepseek-coder-v2:16b-q4_0", "deepseek-coder-v2:16b-q4_0"}, // specific quant
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := FormatModelName(tt.input)
			if got != tt.want {
				t.Errorf("FormatModelName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================
// FormatModelSize
// ============================================================

func TestFormatModelSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1024, "1.0 KB"},
		{1572864, "1.6 MB"},           // 1536 * 1024
		{7600000000, "7.6 GB"},        // 7.6 billion bytes
		{21000000000, "21.0 GB"},      // 21 billion bytes
		{500, "500 B"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := FormatModelSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatModelSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// ============================================================
// RecommendedModels
// ============================================================

func TestRecommendedModels_Anthropic(t *testing.T) {
	t.Parallel()
	models := RecommendedModels(ProviderAnthropic)
	if len(models) == 0 {
		t.Error("should return at least one recommended Anthropic model")
	}
	// Should contain sonnet at minimum
	found := false
	for _, m := range models {
		if m == "sonnet" {
			found = true
		}
	}
	if !found {
		t.Errorf("recommended Anthropic models should include 'sonnet', got %v", models)
	}
}

func TestRecommendedModels_Ollama(t *testing.T) {
	t.Parallel()
	models := RecommendedModels(ProviderOllama)
	if len(models) == 0 {
		t.Error("should return at least one recommended Ollama model")
	}
}

// ============================================================
// ModelInList
// ============================================================

func TestModelInList(t *testing.T) {
	t.Parallel()
	models := []OllamaModel{
		{Name: "qwen3-coder:latest"},
		{Name: "glm-4.7-flash:latest"},
		{Name: "gpt-oss:20b"},
	}

	tests := []struct {
		name string
		want bool
	}{
		{"qwen3-coder:latest", true},
		{"qwen3-coder", true},           // short name matches
		{"glm-4.7-flash", true},
		{"gpt-oss:20b", true},
		{"gpt-oss", true},               // short matches tagged
		{"nonexistent", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ModelInList(tt.name, models)
			if got != tt.want {
				t.Errorf("ModelInList(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ============================================================
// MergeEnvVars
// ============================================================

func TestMergeEnvVars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		existing map[string]string
		provider map[string]string
		wantLen  int
		wantKey  string
		wantVal  string
	}{
		{
			name:     "both nil",
			existing: nil,
			provider: nil,
			wantLen:  0,
		},
		{
			name:     "provider only",
			existing: nil,
			provider: map[string]string{"KEY": "val"},
			wantLen:  1,
			wantKey:  "KEY",
			wantVal:  "val",
		},
		{
			name:     "existing only",
			existing: map[string]string{"MY_VAR": "hello"},
			provider: nil,
			wantLen:  1,
			wantKey:  "MY_VAR",
			wantVal:  "hello",
		},
		{
			name:     "merge without collision",
			existing: map[string]string{"MY_VAR": "hello"},
			provider: map[string]string{"ANTHROPIC_BASE_URL": "http://localhost:11434"},
			wantLen:  2,
			wantKey:  "ANTHROPIC_BASE_URL",
			wantVal:  "http://localhost:11434",
		},
		{
			name:     "provider overwrites on collision",
			existing: map[string]string{"ANTHROPIC_BASE_URL": "old"},
			provider: map[string]string{"ANTHROPIC_BASE_URL": "new"},
			wantLen:  1,
			wantKey:  "ANTHROPIC_BASE_URL",
			wantVal:  "new",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MergeEnvVars(tt.existing, tt.provider)
			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d; result = %v", len(result), tt.wantLen, result)
			}
			if tt.wantKey != "" && result[tt.wantKey] != tt.wantVal {
				t.Errorf("%s = %q, want %q", tt.wantKey, result[tt.wantKey], tt.wantVal)
			}
		})
	}
}

func TestMergeEnvVars_DoesNotMutateInputs(t *testing.T) {
	t.Parallel()
	existing := map[string]string{"A": "1"}
	provider := map[string]string{"B": "2"}

	result := MergeEnvVars(existing, provider)

	// Mutate result — should not affect inputs
	result["C"] = "3"
	if _, ok := existing["C"]; ok {
		t.Error("existing was mutated")
	}
	if _, ok := provider["C"]; ok {
		t.Error("provider was mutated")
	}
}