package executor

import (
	"fmt"
	"strings"

	"github.com/manasm11/forge/internal/state"
)

// BuildExecutionSystemPrompt returns the system prompt used during task execution.
func BuildExecutionSystemPrompt() string {
	return `You are an expert software engineer implementing a specific task.

RULES:
- Implement the task completely and correctly
- Write tests for any new functionality
- Run tests to verify your changes work
- Do not modify files unrelated to this task
- Follow existing code patterns and conventions
- If you encounter issues, explain what went wrong
- Keep changes focused and minimal`
}

// BuildTaskExecutionPrompt produces the full prompt for implementing a single task.
func BuildTaskExecutionPrompt(contextContent string, task state.Task, settings *state.Settings) string {
	var b strings.Builder

	b.WriteString("PROJECT CONTEXT:\n")
	b.WriteString(contextContent)
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "TASK: %s — %s\n", task.ID, task.Title)
	if task.Description != "" {
		b.WriteString(task.Description)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(task.AcceptanceCriteria) > 0 {
		b.WriteString("ACCEPTANCE CRITERIA:\n")
		for _, c := range task.AcceptanceCriteria {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	b.WriteString("INSTRUCTIONS:\n")
	b.WriteString("- Implement this task completely\n")
	b.WriteString("- Write tests if applicable\n")

	if settings != nil {
		if settings.TestCommand != "" {
			fmt.Fprintf(&b, "- Run the test command: %s\n", settings.TestCommand)
			b.WriteString("- Make sure all tests pass\n")
		}
		if settings.BuildCommand != "" {
			fmt.Fprintf(&b, "- Run the build command: %s\n", settings.BuildCommand)
			b.WriteString("- Make sure the build succeeds\n")
		}
	}

	b.WriteString("- Do not modify files unrelated to this task\n")
	b.WriteString("- Follow existing code patterns and conventions\n")

	return b.String()
}

// BuildAllowedTools returns the list of tools Claude is allowed to use during execution.
func BuildAllowedTools(mcpServers []state.MCPServerConfig) []string {
	tools := []string{
		"Bash", "Read", "Write", "Edit",
		"MultiEdit", "TodoRead", "TodoWrite",
	}
	for _, srv := range mcpServers {
		tools = append(tools, "mcp__"+srv.Name)
	}
	return tools
}

// MaxTurnsForTask returns the max turns based on task complexity.
func MaxTurnsForTask(complexity string, config state.MaxTurnsConfig) int {
	switch strings.ToLower(complexity) {
	case "small":
		return config.Small
	case "large":
		return config.Large
	default:
		return config.Medium
	}
}
