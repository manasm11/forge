package generator

import (
	"fmt"
	"strings"

	"github.com/manasm11/forge/internal/state"
)

// GenerateTaskPrompt produces the full prompt for implementing a single task.
func GenerateTaskPrompt(contextContent string, task state.Task, settings *state.Settings) string {
	var b strings.Builder

	b.WriteString("You are implementing a specific task for the project.\n\n")

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

	if settings != nil && settings.TestCommand != "" {
		fmt.Fprintf(&b, "- Run the test command: %s\n", settings.TestCommand)
		b.WriteString("- Make sure all tests pass\n")
	}

	b.WriteString("- Do not modify files unrelated to this task\n")
	b.WriteString("- Follow existing code patterns and conventions\n")
	b.WriteString("- If you encounter issues, explain what went wrong\n")

	return b.String()
}
