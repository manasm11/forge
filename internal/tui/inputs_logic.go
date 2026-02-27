package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/manasm11/forge/internal/state"
)

// InputField represents a single form field in the inputs phase.
type InputField struct {
	Key       string    // settings field name
	Label     string    // displayed label
	Value     string    // current value
	Default   string    // default value
	Required  bool
	FieldType FieldType // text, toggle, number, editor
	HelpText  string    // shown below the field
}

// FieldType represents the type of input field.
type FieldType int

const (
	FieldText   FieldType = iota
	FieldToggle           // yes/no
	FieldNumber
	FieldEditor // opens $EDITOR for long-form input
)

// MCPServer represents an optional MCP server the user can enable.
type MCPServer struct {
	Name        string
	Description string
	Enabled     bool
	Command     string   // e.g., "npx"
	Args        []string // e.g., ["-y", "@upstreamapi/context7-mcp@latest"]
}

// MaxTurnsConfig maps task complexity to max claude turns.
type MaxTurnsConfig struct {
	Small  int `json:"small"`
	Medium int `json:"medium"`
	Large  int `json:"large"`
}

// InferTestCommand guesses the test command from the project snapshot.
func InferTestCommand(snapshot *state.ProjectSnapshot) string {
	if snapshot == nil {
		return ""
	}
	// Check frameworks first for more specific commands
	for _, fw := range snapshot.Frameworks {
		switch fw {
		case "Django":
			return "python manage.py test"
		case "Flutter":
			return "flutter test"
		}
	}
	switch snapshot.Language {
	case "Go":
		return "go test ./..."
	case "JavaScript", "TypeScript":
		return "npm test"
	case "Python":
		return "pytest"
	case "Rust":
		return "cargo test"
	case "Java", "Kotlin":
		return "mvn test"
	case "Ruby":
		return "bundle exec rspec"
	case "Dart":
		return "dart test"
	case "Elixir":
		return "mix test"
	default:
		return ""
	}
}

// InferBuildCommand guesses the build command from the project snapshot.
func InferBuildCommand(snapshot *state.ProjectSnapshot) string {
	if snapshot == nil {
		return ""
	}
	for _, fw := range snapshot.Frameworks {
		if fw == "Flutter" {
			return "flutter build apk"
		}
	}
	switch snapshot.Language {
	case "Go":
		return "go build ./..."
	case "JavaScript", "TypeScript":
		return "npm run build"
	case "Rust":
		return "cargo build"
	case "Java", "Kotlin":
		return "mvn package"
	default:
		return ""
	}
}

// DefaultInputFields returns the initial form fields with smart defaults.
func DefaultInputFields(snapshot *state.ProjectSnapshot) []InputField {
	return []InputField{
		{
			Key:       "test_command",
			Label:     "Test Command",
			Default:   InferTestCommand(snapshot),
			Required:  true,
			FieldType: FieldText,
			HelpText:  "Command to run tests after each task",
		},
		{
			Key:       "build_command",
			Label:     "Build Command (optional)",
			Default:   InferBuildCommand(snapshot),
			Required:  false,
			FieldType: FieldText,
			HelpText:  "Command to verify build succeeds",
		},
		{
			Key:       "branch_pattern",
			Label:     "Branch Pattern",
			Default:   "forge/task-{id}",
			Required:  true,
			FieldType: FieldText,
			HelpText:  "{id} is replaced with the task ID",
		},
		{
			Key:       "max_retries",
			Label:     "Max Retries per Task",
			Default:   "3",
			Required:  false,
			FieldType: FieldNumber,
			HelpText:  "How many times to retry a failed task",
		},
		{
			Key:       "auto_pr",
			Label:     "Auto-create Pull Requests",
			Default:   "true",
			Required:  false,
			FieldType: FieldToggle,
			HelpText:  "Create PRs automatically after pushing",
		},
		{
			Key:       "claude_model",
			Label:     "Claude Model for Execution",
			Default:   "sonnet",
			Required:  true,
			FieldType: FieldText,
			HelpText:  "Model name: sonnet, opus, etc.",
		},
		{
			Key:       "extra_context",
			Label:     "Additional Context (optional)",
			Default:   "",
			Required:  false,
			FieldType: FieldEditor,
			HelpText:  "Press Enter to open editor — add any info Claude should know",
		},
	}
}

// DefaultMCPServers returns the available MCP servers with defaults.
func DefaultMCPServers() []MCPServer {
	return []MCPServer{
		{
			Name:        "context7",
			Description: "Up-to-date library documentation",
			Enabled:     true,
			Command:     "npx",
			Args:        []string{"-y", "@upstreamapi/context7-mcp@latest"},
		},
		{
			Name:        "web_search",
			Description: "Web search during implementation",
			Enabled:     false,
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/web-search-mcp"},
		},
	}
}

// DefaultMaxTurns returns the default max turns per complexity.
func DefaultMaxTurns() MaxTurnsConfig {
	return MaxTurnsConfig{Small: 20, Medium: 35, Large: 50}
}

// ValidateSettings checks that all required fields have values
// and that values are valid (e.g., max retries is a positive number).
func ValidateSettings(fields []InputField) []string {
	var errs []string
	for _, f := range fields {
		val := f.Value
		if val == "" {
			val = f.Default
		}

		// Required check
		if f.Required && val == "" {
			errs = append(errs, fmt.Sprintf("%s is required", f.Label))
			continue
		}

		// Number validation
		if f.FieldType == FieldNumber && val != "" {
			n, err := strconv.Atoi(val)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s must be a non-negative number", f.Label))
			} else if n < 0 {
				errs = append(errs, fmt.Sprintf("%s must be a non-negative number", f.Label))
			}
		}

		// Branch pattern must contain {id}
		if f.Key == "branch_pattern" && val != "" && !strings.Contains(val, "{id}") {
			errs = append(errs, "Branch Pattern must contain {id} placeholder")
		}
	}
	return errs
}

// BuildSettingsFromFields converts form fields into a state.Settings struct.
func BuildSettingsFromFields(fields []InputField, mcpServers []MCPServer, maxTurns MaxTurnsConfig) *state.Settings {
	s := &state.Settings{}

	fieldMap := make(map[string]string)
	for _, f := range fields {
		val := f.Value
		if val == "" {
			val = f.Default
		}
		fieldMap[f.Key] = val
	}

	s.TestCommand = fieldMap["test_command"]
	s.BuildCommand = fieldMap["build_command"]
	s.BranchPattern = fieldMap["branch_pattern"]
	s.AutoPR = fieldMap["auto_pr"] == "true"
	s.ClaudeModel = fieldMap["claude_model"]
	s.ExtraContext = fieldMap["extra_context"]

	if v, err := strconv.Atoi(fieldMap["max_retries"]); err == nil {
		s.MaxRetries = v
	}

	s.MaxTurns = state.MaxTurnsConfig{
		Small:  maxTurns.Small,
		Medium: maxTurns.Medium,
		Large:  maxTurns.Large,
	}

	// Only include enabled MCP servers
	for _, srv := range mcpServers {
		if srv.Enabled {
			s.MCPServers = append(s.MCPServers, state.MCPServerConfig{
				Name:    srv.Name,
				Command: srv.Command,
				Args:    srv.Args,
			})
		}
	}

	return s
}
