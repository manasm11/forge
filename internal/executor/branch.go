package executor

import (
	"fmt"
	"strings"
)

// ResolveBranchName replaces {id} in the pattern with the task ID.
func ResolveBranchName(pattern, taskID string) string {
	return strings.ReplaceAll(pattern, "{id}", taskID)
}

// SanitizeBranchName cleans a branch name for git compatibility.
// Removes invalid characters, leading dots, trailing .lock, and consecutive dots.
func SanitizeBranchName(name string) string {
	// Replace invalid git ref characters with hyphens
	var b strings.Builder
	for _, r := range name {
		switch r {
		case ' ', '~', '^', ':', '\\', '?', '*', '[', ']', '@', '{', '}':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	result := b.String()

	// Collapse consecutive dots into a single dot
	for strings.Contains(result, "..") {
		result = strings.ReplaceAll(result, "..", ".")
	}

	// Remove leading dots
	result = strings.TrimLeft(result, ".")

	// Remove trailing .lock
	result = strings.TrimSuffix(result, ".lock")

	return result
}

// CommitMessage formats the commit message for a task.
func CommitMessage(taskID, title string) string {
	return fmt.Sprintf("forge: %s — %s", taskID, title)
}
