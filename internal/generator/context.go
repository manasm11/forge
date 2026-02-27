package generator

import (
	"fmt"
	"strings"

	"github.com/manasm11/forge/internal/state"
)

// GenerateContextFile produces the contents of .forge/context.md.
func GenerateContextFile(s *state.State) string {
	var b strings.Builder

	// Header
	name := s.ProjectName
	if name == "" {
		name = "Project"
	}
	fmt.Fprintf(&b, "# Project Context: %s\n\n", name)

	// Tech stack from snapshot
	if s.Snapshot != nil {
		if s.Snapshot.Language != "" {
			fmt.Fprintf(&b, "## Tech Stack\n")
			parts := []string{s.Snapshot.Language}
			parts = append(parts, s.Snapshot.Frameworks...)
			fmt.Fprintf(&b, "%s\n\n", strings.Join(parts, ", "))
		}
	}

	// Settings
	if s.Settings != nil {
		b.WriteString("## Commands\n")
		if s.Settings.TestCommand != "" {
			fmt.Fprintf(&b, "Run tests: `%s`\n", s.Settings.TestCommand)
		}
		if s.Settings.BuildCommand != "" {
			fmt.Fprintf(&b, "Build: `%s`\n", s.Settings.BuildCommand)
		}
		b.WriteString("\n")
	}

	// Completed tasks
	var completed, pending []state.Task
	for _, t := range s.Tasks {
		switch t.Status {
		case state.TaskDone:
			completed = append(completed, t)
		case state.TaskPending, state.TaskInProgress:
			pending = append(pending, t)
		}
	}

	if len(completed) > 0 {
		b.WriteString("## Completed Work\n")
		for _, t := range completed {
			fmt.Fprintf(&b, "- %s: %s ✅\n", t.ID, t.Title)
		}
		b.WriteString("\n")
	}

	// Remaining tasks
	if len(pending) > 0 {
		b.WriteString("## Remaining Tasks\n")
		for _, t := range pending {
			line := fmt.Sprintf("- %s: %s", t.ID, t.Title)
			if len(t.DependsOn) > 0 {
				line += fmt.Sprintf(" (depends on: %s)", strings.Join(t.DependsOn, ", "))
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// Extra context
	if s.Settings != nil && s.Settings.ExtraContext != "" {
		fmt.Fprintf(&b, "## Additional Context\n%s\n\n", s.Settings.ExtraContext)
	}

	// Instructions
	b.WriteString("## Instructions\n")
	b.WriteString("- Implement only the specific task assigned to you\n")
	b.WriteString("- Do not modify files unrelated to the current task\n")
	b.WriteString("- Write tests for your implementation\n")
	if s.Settings != nil && s.Settings.TestCommand != "" {
		b.WriteString("- Run the test command after making changes\n")
	}
	b.WriteString("- Follow existing code patterns and conventions\n")

	return b.String()
}

// GenerateClaudeMD produces the contents of CLAUDE.md for the project.
func GenerateClaudeMD(s *state.State) string {
	var b strings.Builder

	name := s.ProjectName
	if name == "" {
		name = "Project"
	}
	fmt.Fprintf(&b, "# %s\n\n", name)

	// Tech stack
	if s.Snapshot != nil && s.Snapshot.Language != "" {
		b.WriteString("## Tech Stack\n")
		fmt.Fprintf(&b, "- %s\n", s.Snapshot.Language)
		for _, fw := range s.Snapshot.Frameworks {
			fmt.Fprintf(&b, "- %s\n", fw)
		}
		b.WriteString("\n")
	}

	// Project structure
	if s.Snapshot != nil && s.Snapshot.Structure != "" {
		b.WriteString("## Project Structure\n```\n")
		b.WriteString(s.Snapshot.Structure)
		b.WriteString("\n```\n\n")
	}

	// Commands
	if s.Settings != nil {
		b.WriteString("## Testing\n")
		if s.Settings.TestCommand != "" {
			fmt.Fprintf(&b, "Run tests: `%s`\n", s.Settings.TestCommand)
		}
		if s.Settings.BuildCommand != "" {
			fmt.Fprintf(&b, "Build: `%s`\n", s.Settings.BuildCommand)
		}
		b.WriteString("\n")
	}

	// Conventions
	b.WriteString("## Conventions\n")
	b.WriteString("- Follow existing code patterns in the project\n")
	b.WriteString("- Handle errors explicitly\n")
	b.WriteString("- Write tests for new functionality\n")

	return b.String()
}
