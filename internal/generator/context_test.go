package generator

import (
	"strings"
	"testing"
	"time"

	"github.com/manasm11/forge/internal/state"
)

// ============================================================
// GenerateContextFile
// ============================================================

func TestGenerateContextFile(t *testing.T) {
	t.Parallel()
	now := time.Now()
	s := &state.State{
		ProjectName: "inventory-api",
		PlanVersion: 2,
		Snapshot: &state.ProjectSnapshot{
			Language:   "Go",
			Frameworks: []string{"Gin", "GORM"},
		},
		Settings: &state.Settings{
			TestCommand:   "go test ./...",
			BuildCommand:  "go build ./...",
			BranchPattern: "forge/task-{id}",
			ExtraContext:  "Use PostgreSQL for production, SQLite for tests",
		},
		Tasks: []state.Task{
			{ID: "task-001", Title: "Init project", Status: state.TaskDone, CompletedAt: &now},
			{ID: "task-002", Title: "Add auth", Status: state.TaskPending,
				Description: "JWT auth", AcceptanceCriteria: []string{"login works"}},
			{ID: "task-003", Title: "Add API", Status: state.TaskPending,
				DependsOn: []string{"task-002"}},
		},
	}

	content := GenerateContextFile(s)

	mustContain := []string{
		"inventory-api",
		"Go",
		"Gin",
		"go test ./...",
		"go build ./...",
		"PostgreSQL",
		"task-001",
		"Init project",
		"task-002",
		"Add auth",
	}
	for _, want := range mustContain {
		if !strings.Contains(content, want) {
			t.Errorf("context file missing %q", want)
		}
	}
}

func TestGenerateContextFile_MinimalState(t *testing.T) {
	t.Parallel()
	s := &state.State{
		ProjectName: "test",
		Settings:    &state.Settings{TestCommand: "make test"},
		Tasks: []state.Task{
			{ID: "task-001", Title: "Do thing", Status: state.TaskPending},
		},
	}

	content := GenerateContextFile(s)

	if content == "" {
		t.Error("should produce non-empty content even with minimal state")
	}
	if !strings.Contains(content, "make test") {
		t.Error("should contain test command")
	}
}

func TestGenerateContextFile_NoSettings(t *testing.T) {
	t.Parallel()
	s := &state.State{
		ProjectName: "test",
		Tasks:       []state.Task{{ID: "task-001", Title: "X", Status: state.TaskPending}},
	}

	content := GenerateContextFile(s)

	if content == "" {
		t.Error("should handle nil settings gracefully")
	}
}

// ============================================================
// GenerateClaudeMD
// ============================================================

func TestGenerateClaudeMD(t *testing.T) {
	t.Parallel()
	s := &state.State{
		ProjectName: "inventory-api",
		Snapshot: &state.ProjectSnapshot{
			Language:   "Go",
			Frameworks: []string{"Gin", "GORM"},
			Structure:  "cmd/\n  server/\n    main.go\ninternal/\n  handlers/",
		},
		Settings: &state.Settings{
			TestCommand:  "go test ./...",
			BuildCommand: "go build ./...",
		},
	}

	content := GenerateClaudeMD(s)

	mustContain := []string{
		"inventory-api",
		"Go",
		"Gin",
		"go test",
		"go build",
	}
	for _, want := range mustContain {
		if !strings.Contains(content, want) {
			t.Errorf("CLAUDE.md missing %q", want)
		}
	}

	if !strings.Contains(content, "cmd/") {
		t.Error("should include project structure")
	}
}

func TestGenerateClaudeMD_NoSnapshot(t *testing.T) {
	t.Parallel()
	s := &state.State{
		ProjectName: "new-project",
		Settings:    &state.Settings{TestCommand: "go test ./..."},
	}

	content := GenerateClaudeMD(s)

	if content == "" {
		t.Error("should produce content without snapshot")
	}
	if !strings.Contains(content, "new-project") {
		t.Error("should contain project name")
	}
}

// ============================================================
// GenerateMCPConfig
// ============================================================

func TestGenerateMCPConfig(t *testing.T) {
	t.Parallel()
	servers := []MCPServer{
		{
			Name:    "context7",
			Enabled: true,
			Command: "npx",
			Args:    []string{"-y", "@upstreamapi/context7-mcp@latest"},
		},
		{
			Name:    "web_search",
			Enabled: false,
			Command: "npx",
			Args:    []string{"-y", "@anthropic/web-search-mcp"},
		},
	}

	config := GenerateMCPConfig(servers)

	if !strings.HasPrefix(strings.TrimSpace(config), "{") {
		t.Error("should produce valid JSON")
	}

	if !strings.Contains(config, "context7") {
		t.Error("should contain enabled context7 server")
	}

	if strings.Contains(config, "web_search") {
		t.Error("should not contain disabled web_search server")
	}

	if !strings.Contains(config, "npx") {
		t.Error("should contain npx command")
	}
}

func TestGenerateMCPConfig_NoEnabledServers(t *testing.T) {
	t.Parallel()
	servers := []MCPServer{
		{Name: "context7", Enabled: false},
	}

	config := GenerateMCPConfig(servers)

	if strings.Contains(config, "context7") {
		t.Error("should not contain disabled servers")
	}
}

func TestGenerateMCPConfig_AllEnabled(t *testing.T) {
	t.Parallel()
	servers := []MCPServer{
		{Name: "context7", Enabled: true, Command: "npx", Args: []string{"-y", "ctx7"}},
		{Name: "web_search", Enabled: true, Command: "npx", Args: []string{"-y", "ws"}},
	}

	config := GenerateMCPConfig(servers)

	if !strings.Contains(config, "context7") || !strings.Contains(config, "web_search") {
		t.Error("should contain all enabled servers")
	}
}

// ============================================================
// GenerateTaskPrompt
// ============================================================

func TestGenerateTaskPrompt(t *testing.T) {
	t.Parallel()
	contextContent := "# Project: test-api\nTech: Go, Gin"
	task := state.Task{
		ID:                 "task-003",
		Title:              "Add user authentication",
		Description:        "Implement JWT-based auth with login and register endpoints",
		AcceptanceCriteria: []string{"POST /register works", "POST /login returns token"},
		Complexity:         "medium",
		DependsOn:          []string{"task-001"},
	}
	settings := &state.Settings{
		TestCommand: "go test ./...",
	}

	prompt := GenerateTaskPrompt(contextContent, task, settings)

	mustContain := []string{
		"test-api",
		"task-003",
		"Add user authentication",
		"JWT-based auth",
		"POST /register works",
		"POST /login returns token",
		"go test ./...",
		"Do not modify files unrelated",
	}
	for _, want := range mustContain {
		if !strings.Contains(prompt, want) {
			t.Errorf("task prompt missing %q", want)
		}
	}
}

func TestGenerateTaskPrompt_NoTestCommand(t *testing.T) {
	t.Parallel()
	task := state.Task{
		ID: "task-001", Title: "Init", Description: "Setup",
		AcceptanceCriteria: []string{"compiles"},
	}

	prompt := GenerateTaskPrompt("context", task, &state.Settings{})

	if strings.Contains(prompt, "Run the test command: \n") {
		t.Error("should handle empty test command gracefully")
	}
}
