package executor

import (
	"strings"
	"testing"

	"github.com/manasm11/forge/internal/state"
)

func TestBuildExecutionSystemPrompt(t *testing.T) {
	t.Parallel()
	prompt := BuildExecutionSystemPrompt()

	mustContain := []string{
		"implement",
		"test",
		"do not modify files unrelated",
	}
	for _, s := range mustContain {
		if !strings.Contains(strings.ToLower(prompt), s) {
			t.Errorf("system prompt missing %q", s)
		}
	}
}

func TestBuildTaskExecutionPrompt(t *testing.T) {
	t.Parallel()
	task := state.Task{
		ID:                 "task-003",
		Title:              "Add user auth",
		Description:        "Implement JWT-based auth",
		AcceptanceCriteria: []string{"login works", "token validates"},
		Complexity:         "medium",
		DependsOn:          []string{"task-001"},
	}
	contextContent := "# Project: test\nTech: Go"
	settings := &state.Settings{
		TestCommand:  "go test ./...",
		BuildCommand: "go build ./...",
	}

	prompt := BuildTaskExecutionPrompt(contextContent, task, settings)

	mustContain := []string{
		"task-003",
		"Add user auth",
		"JWT-based auth",
		"login works",
		"token validates",
		"go test ./...",
		"go build ./...",
	}
	for _, s := range mustContain {
		if !strings.Contains(prompt, s) {
			t.Errorf("task prompt missing %q", s)
		}
	}
}

func TestBuildAllowedTools(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		mcpServers []state.MCPServerConfig
		mustHave   []string
	}{
		{
			name:       "no MCP servers",
			mcpServers: nil,
			mustHave:   []string{"Bash", "Read", "Write", "Edit"},
		},
		{
			name: "with context7",
			mcpServers: []state.MCPServerConfig{
				{Name: "context7"},
			},
			mustHave: []string{"Bash", "Read", "Write", "Edit", "mcp__context7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tools := BuildAllowedTools(tt.mcpServers)
			for _, want := range tt.mustHave {
				found := false
				for _, tool := range tools {
					if strings.Contains(tool, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("allowed tools missing %q, got: %v", want, tools)
				}
			}
		})
	}
}

func TestMaxTurnsForTask(t *testing.T) {
	t.Parallel()
	config := state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50}
	tests := []struct {
		complexity string
		want       int
	}{
		{"small", 20},
		{"medium", 35},
		{"large", 50},
		{"", 35},
		{"huge", 35},
	}
	for _, tt := range tests {
		t.Run(tt.complexity, func(t *testing.T) {
			t.Parallel()
			got := MaxTurnsForTask(tt.complexity, config)
			if got != tt.want {
				t.Errorf("MaxTurnsForTask(%q) = %d, want %d", tt.complexity, got, tt.want)
			}
		})
	}
}
