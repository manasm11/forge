package tui

import (
	"testing"

	"github.com/manasm11/forge/internal/provider"
	"github.com/manasm11/forge/internal/state"
)

// ============================================================
// InferTestCommand
// ============================================================

func TestInferTestCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		snapshot *state.ProjectSnapshot
		want     string
	}{
		{
			name:     "Go project",
			snapshot: &state.ProjectSnapshot{Language: "Go"},
			want:     "go test ./...",
		},
		{
			name:     "JavaScript project",
			snapshot: &state.ProjectSnapshot{Language: "JavaScript"},
			want:     "npm test",
		},
		{
			name:     "TypeScript project",
			snapshot: &state.ProjectSnapshot{Language: "TypeScript"},
			want:     "npm test",
		},
		{
			name:     "Python project",
			snapshot: &state.ProjectSnapshot{Language: "Python"},
			want:     "pytest",
		},
		{
			name:     "Rust project",
			snapshot: &state.ProjectSnapshot{Language: "Rust"},
			want:     "cargo test",
		},
		{
			name: "Python with Django",
			snapshot: &state.ProjectSnapshot{
				Language:   "Python",
				Frameworks: []string{"Django"},
			},
			want: "python manage.py test",
		},
		{
			name:     "unknown language",
			snapshot: &state.ProjectSnapshot{Language: "Brainfuck"},
			want:     "",
		},
		{
			name:     "nil snapshot",
			snapshot: nil,
			want:     "",
		},
		{
			name: "Flutter project",
			snapshot: &state.ProjectSnapshot{
				Language:   "Dart",
				Frameworks: []string{"Flutter"},
			},
			want: "flutter test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InferTestCommand(tt.snapshot)
			if got != tt.want {
				t.Errorf("InferTestCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// InferBuildCommand
// ============================================================

func TestInferBuildCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		snapshot *state.ProjectSnapshot
		want     string
	}{
		{
			name:     "Go project",
			snapshot: &state.ProjectSnapshot{Language: "Go"},
			want:     "go build ./...",
		},
		{
			name:     "JavaScript project",
			snapshot: &state.ProjectSnapshot{Language: "JavaScript"},
			want:     "npm run build",
		},
		{
			name:     "Rust project",
			snapshot: &state.ProjectSnapshot{Language: "Rust"},
			want:     "cargo build",
		},
		{
			name:     "Python project — no build",
			snapshot: &state.ProjectSnapshot{Language: "Python"},
			want:     "",
		},
		{
			name:     "nil snapshot",
			snapshot: nil,
			want:     "",
		},
		{
			name: "Flutter project",
			snapshot: &state.ProjectSnapshot{
				Language:   "Dart",
				Frameworks: []string{"Flutter"},
			},
			want: "flutter build apk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InferBuildCommand(tt.snapshot)
			if got != tt.want {
				t.Errorf("InferBuildCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// DefaultInputFields
// ============================================================

func TestDefaultInputFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		snapshot          *state.ProjectSnapshot
		wantTestDefault   string
		wantBranchDefault string
	}{
		{
			name:              "Go project gets go test default",
			snapshot:          &state.ProjectSnapshot{Language: "Go"},
			wantTestDefault:   "go test ./...",
			wantBranchDefault: "forge/task-{id}",
		},
		{
			name:              "nil snapshot gets empty test default",
			snapshot:          nil,
			wantTestDefault:   "",
			wantBranchDefault: "forge/task-{id}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fields := DefaultInputFields(tt.snapshot)

			var testField, branchField *InputField
			for i := range fields {
				switch fields[i].Key {
				case "test_command":
					testField = &fields[i]
				case "branch_pattern":
					branchField = &fields[i]
				}
			}

			if testField == nil {
				t.Fatal("test_command field not found")
			}
			if testField.Default != tt.wantTestDefault {
				t.Errorf("test_command default = %q, want %q", testField.Default, tt.wantTestDefault)
			}
			if branchField == nil {
				t.Fatal("branch_pattern field not found")
			}
			if branchField.Default != tt.wantBranchDefault {
				t.Errorf("branch_pattern default = %q, want %q", branchField.Default, tt.wantBranchDefault)
			}
		})
	}
}

func TestDefaultInputFields_AllFieldsPresent(t *testing.T) {
	t.Parallel()
	fields := DefaultInputFields(nil)

	requiredKeys := []string{
		"test_command", "build_command", "branch_pattern",
		"max_retries", "auto_pr", "claude_model", "extra_context",
	}
	fieldKeys := make(map[string]bool)
	for _, f := range fields {
		fieldKeys[f.Key] = true
	}
	for _, key := range requiredKeys {
		if !fieldKeys[key] {
			t.Errorf("missing field %q", key)
		}
	}
}

// ============================================================
// ValidateSettings
// ============================================================

func TestValidateSettings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		fields     []InputField
		wantErrors int
	}{
		{
			name: "all valid",
			fields: []InputField{
				{Key: "test_command", Value: "go test ./...", Required: true},
				{Key: "branch_pattern", Value: "forge/task-{id}", Required: true},
				{Key: "max_retries", Value: "3", FieldType: FieldNumber},
				{Key: "auto_pr", Value: "true", FieldType: FieldToggle},
			},
			wantErrors: 0,
		},
		{
			name: "missing required field",
			fields: []InputField{
				{Key: "test_command", Value: "", Required: true},
				{Key: "branch_pattern", Value: "forge/task-{id}", Required: true},
			},
			wantErrors: 1,
		},
		{
			name: "invalid number",
			fields: []InputField{
				{Key: "max_retries", Value: "abc", FieldType: FieldNumber},
			},
			wantErrors: 1,
		},
		{
			name: "negative number",
			fields: []InputField{
				{Key: "max_retries", Value: "-1", FieldType: FieldNumber},
			},
			wantErrors: 1,
		},
		{
			name: "zero retries is valid",
			fields: []InputField{
				{Key: "max_retries", Value: "0", FieldType: FieldNumber},
			},
			wantErrors: 0,
		},
		{
			name: "branch pattern missing {id} placeholder",
			fields: []InputField{
				{Key: "branch_pattern", Value: "forge/task", Required: true},
			},
			wantErrors: 1,
		},
		{
			name: "non-required empty field is ok",
			fields: []InputField{
				{Key: "build_command", Value: "", Required: false},
			},
			wantErrors: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errors := ValidateSettings(tt.fields)
			if len(errors) != tt.wantErrors {
				t.Errorf("errors count = %d, want %d: %v", len(errors), tt.wantErrors, errors)
			}
		})
	}
}

// ============================================================
// BuildSettingsFromFields
// ============================================================

func TestBuildSettingsFromFields(t *testing.T) {
	t.Parallel()
	fields := []InputField{
		{Key: "test_command", Value: "go test ./..."},
		{Key: "build_command", Value: "go build ./..."},
		{Key: "branch_pattern", Value: "forge/task-{id}"},
		{Key: "max_retries", Value: "5"},
		{Key: "auto_pr", Value: "true"},
		{Key: "claude_model", Value: "sonnet"},
		{Key: "extra_context", Value: "Use Gin for HTTP"},
	}
	mcpServers := []MCPServer{
		{Name: "context7", Enabled: true, Command: "npx", Args: []string{"-y", "@upstreamapi/context7-mcp@latest"}},
		{Name: "web_search", Enabled: false},
	}
	maxTurns := MaxTurnsConfig{Small: 20, Medium: 35, Large: 50}

	settings := BuildSettingsFromFields(fields, mcpServers, maxTurns)

	if settings.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q", settings.TestCommand)
	}
	if settings.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q", settings.BuildCommand)
	}
	if settings.BranchPattern != "forge/task-{id}" {
		t.Errorf("BranchPattern = %q", settings.BranchPattern)
	}
	if settings.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d", settings.MaxRetries)
	}
	if settings.AutoPR != true {
		t.Errorf("AutoPR = %v", settings.AutoPR)
	}
	if settings.ExtraContext != "Use Gin for HTTP" {
		t.Errorf("ExtraContext = %q", settings.ExtraContext)
	}
	if settings.ClaudeModel != "sonnet" {
		t.Errorf("ClaudeModel = %q", settings.ClaudeModel)
	}
	if settings.MaxTurns.Small != 20 || settings.MaxTurns.Medium != 35 || settings.MaxTurns.Large != 50 {
		t.Errorf("MaxTurns = %+v", settings.MaxTurns)
	}
	// Only enabled MCP servers should be in config
	if len(settings.MCPServers) != 1 {
		t.Errorf("MCPServers count = %d, want 1", len(settings.MCPServers))
	}
	if len(settings.MCPServers) > 0 && settings.MCPServers[0].Name != "context7" {
		t.Errorf("MCPServers[0].Name = %q", settings.MCPServers[0].Name)
	}
}

// ============================================================
// DefaultMCPServers
// ============================================================

func TestDefaultMCPServers(t *testing.T) {
	t.Parallel()
	servers := DefaultMCPServers()

	if len(servers) < 1 {
		t.Fatal("should have at least one MCP server option")
	}

	var found bool
	for _, s := range servers {
		if s.Name == "context7" {
			found = true
			if !s.Enabled {
				t.Error("context7 should be enabled by default")
			}
			if s.Command == "" {
				t.Error("context7 should have a command")
			}
		}
	}
	if !found {
		t.Error("context7 MCP server not found in defaults")
	}
}

// ============================================================
// DefaultMaxTurns
// ============================================================

func TestDefaultMaxTurns(t *testing.T) {
	t.Parallel()
	mt := DefaultMaxTurns()

	if mt.Small <= 0 || mt.Small >= mt.Medium {
		t.Errorf("Small = %d, should be positive and less than Medium", mt.Small)
	}
	if mt.Medium <= mt.Small || mt.Medium >= mt.Large {
		t.Errorf("Medium = %d, should be between Small and Large", mt.Medium)
	}
	if mt.Large <= mt.Medium {
		t.Errorf("Large = %d, should be greater than Medium", mt.Large)
	}
}

// ============================================================
// Provider detection + field integration
// ============================================================

func TestDefaultProviderConfig_NoOllama(t *testing.T) {
	t.Parallel()
	// When Ollama is not detected, default to Anthropic
	cfg := DefaultProviderConfig(nil)

	if cfg.Type != provider.ProviderAnthropic {
		t.Errorf("Type = %q, want anthropic", cfg.Type)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", cfg.Model)
	}
}

func TestDefaultProviderConfig_WithOllama(t *testing.T) {
	t.Parallel()
	// Even when Ollama is detected, default is still Anthropic
	// (user must explicitly opt in)
	status := &provider.OllamaStatus{
		Available: true,
		URL:       "http://localhost:11434",
		Models: []provider.OllamaModel{
			{Name: "qwen3-coder:latest"},
		},
	}
	cfg := DefaultProviderConfig(status)

	if cfg.Type != provider.ProviderAnthropic {
		t.Errorf("should still default to Anthropic, got %q", cfg.Type)
	}
}

func TestBuildProviderConfigFromFields_Anthropic(t *testing.T) {
	t.Parallel()
	fields := map[string]string{
		"provider_type": "anthropic",
		"claude_model":  "opus",
		"ollama_url":    "",
	}
	cfg := BuildProviderConfigFromFields(fields)

	if cfg.Type != provider.ProviderAnthropic {
		t.Errorf("Type = %q", cfg.Type)
	}
	if cfg.Model != "opus" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.OllamaURL != "" {
		t.Errorf("OllamaURL should be empty for Anthropic, got %q", cfg.OllamaURL)
	}
}

func TestBuildProviderConfigFromFields_Ollama(t *testing.T) {
	t.Parallel()
	fields := map[string]string{
		"provider_type": "ollama",
		"claude_model":  "qwen3-coder",
		"ollama_url":    "http://myserver:11434",
	}
	cfg := BuildProviderConfigFromFields(fields)

	if cfg.Type != provider.ProviderOllama {
		t.Errorf("Type = %q", cfg.Type)
	}
	if cfg.Model != "qwen3-coder" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.OllamaURL != "http://myserver:11434" {
		t.Errorf("OllamaURL = %q", cfg.OllamaURL)
	}
}

func TestBuildProviderConfigFromFields_Ollama_EmptyURL(t *testing.T) {
	t.Parallel()
	fields := map[string]string{
		"provider_type": "ollama",
		"claude_model":  "qwen3-coder",
		"ollama_url":    "",
	}
	cfg := BuildProviderConfigFromFields(fields)

	if cfg.OllamaURL != provider.DefaultOllamaURL() {
		t.Errorf("should default to %q, got %q", provider.DefaultOllamaURL(), cfg.OllamaURL)
	}
}

func TestOllamaModelNames_FromStatus(t *testing.T) {
	t.Parallel()
	status := &provider.OllamaStatus{
		Available: true,
		Models: []provider.OllamaModel{
			{Name: "qwen3-coder:latest"},
			{Name: "glm-4.7-flash:latest"},
			{Name: "gpt-oss:20b"},
		},
	}

	names := OllamaModelNames(status)

	if len(names) != 3 {
		t.Fatalf("count = %d, want 3", len(names))
	}
	// Should be display-formatted (no ":latest")
	if names[0] != "qwen3-coder" {
		t.Errorf("names[0] = %q", names[0])
	}
	if names[2] != "gpt-oss:20b" {
		t.Errorf("names[2] = %q, tag should be preserved", names[2])
	}
}

func TestOllamaModelNames_NilStatus(t *testing.T) {
	t.Parallel()
	names := OllamaModelNames(nil)
	if len(names) != 0 {
		t.Errorf("should be empty for nil, got %v", names)
	}
}

func TestOllamaModelNames_NotAvailable(t *testing.T) {
	t.Parallel()
	status := &provider.OllamaStatus{Available: false}
	names := OllamaModelNames(status)
	if len(names) != 0 {
		t.Errorf("should be empty for unavailable, got %v", names)
	}
}

// ============================================================
// Settings round-trip with provider config
// ============================================================

func TestBuildSettingsFromFields_IncludesProvider(t *testing.T) {
	t.Parallel()
	// Existing test from M7, extended to verify provider config persists
	fields := []InputField{
		{Key: "test_command", Value: "go test ./..."},
		{Key: "build_command", Value: "go build ./..."},
		{Key: "branch_pattern", Value: "forge/task-{id}"},
		{Key: "max_retries", Value: "3"},
		{Key: "auto_pr", Value: "true"},
		{Key: "claude_model", Value: "qwen3-coder"},
	}

	providerCfg := provider.Config{
		Type:      provider.ProviderOllama,
		Model:     "qwen3-coder",
		OllamaURL: "http://localhost:11434",
	}

	settings := BuildSettingsFromFieldsWithProvider(fields, nil, DefaultMaxTurns(), providerCfg)

	if settings.Provider.Type != provider.ProviderOllama {
		t.Errorf("Provider.Type = %q", settings.Provider.Type)
	}
	if settings.Provider.Model != "qwen3-coder" {
		t.Errorf("Provider.Model = %q", settings.Provider.Model)
	}
	if settings.ClaudeModel != "qwen3-coder" {
		t.Errorf("ClaudeModel = %q, should match provider model", settings.ClaudeModel)
	}
}
