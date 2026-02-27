package tui

import (
	"fmt"
	"strings"

	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/state"
)

// ReplanContext holds everything needed to start a replanning session.
type ReplanContext struct {
	PlanVersion         int
	ConversationHistory []state.ConversationMsg
	SystemContext       string // injected context about current task state
	CompletedCount      int
	PendingCount        int
	FailedCount         int
}

// BuildReplanContext prepares the full context for a replanning session.
func BuildReplanContext(s *state.State) ReplanContext {
	return ReplanContext{
		PlanVersion:         s.PlanVersion,
		ConversationHistory: s.ConversationHistory,
		SystemContext:       s.GenerateReplanContext(),
		CompletedCount:      len(s.CompletedTasks()),
		PendingCount:        len(s.PendingTasks()),
		FailedCount:         len(s.FailedTasks()),
	}
}

// BuildReplanSystemMessage creates the system message shown to the user
// when they enter replanning mode.
func BuildReplanSystemMessage(ctx ReplanContext) string {
	var b strings.Builder
	b.WriteString("Welcome back to planning! \u2692\n\n")
	fmt.Fprintf(&b, "You have %d completed tasks and %d pending tasks.\n", ctx.CompletedCount, ctx.PendingCount)
	if ctx.FailedCount > 0 {
		fmt.Fprintf(&b, "%d failed and may need redesigning.\n", ctx.FailedCount)
	}
	b.WriteString("Tell me what changes you'd like to make to the plan.\n\n")
	b.WriteString("Commands: /done \u00b7 /summary \u00b7 /restart")
	return b.String()
}

// BuildReplanPrompt constructs the full system prompt for Claude,
// combining the replanning prompt template with the task state context.
func BuildReplanPrompt(ctx ReplanContext) string {
	return fmt.Sprintf(claude.ReplanningPrompt, ctx.SystemContext)
}

// ValidatePlanUpdate checks a PlanUpdateJSON for logical errors before applying.
// Returns a list of warnings (non-fatal) and an error (fatal).
func ValidatePlanUpdate(s *state.State, update *claude.PlanUpdateJSON) (warnings []string, err error) {
	// Build a map of existing task IDs to their status
	taskMap := make(map[string]*state.Task, len(s.Tasks))
	for i := range s.Tasks {
		taskMap[s.Tasks[i].ID] = &s.Tasks[i]
	}

	// Track seen IDs to detect duplicates
	seen := make(map[string]bool)

	for _, t := range update.Tasks {
		switch t.Action {
		case "keep", "modify", "remove":
			if t.ID == "" {
				return warnings, fmt.Errorf("action %q requires a task ID", t.Action)
			}
			// Check for duplicates
			if seen[t.ID] {
				return warnings, fmt.Errorf("duplicate task ID %q in update", t.ID)
			}
			seen[t.ID] = true

			// Check task exists
			existing, ok := taskMap[t.ID]
			if !ok {
				return warnings, fmt.Errorf("task %q not found", t.ID)
			}

			// "modify" and "remove" cannot target completed tasks
			if (t.Action == "modify" || t.Action == "remove") && existing.Status == state.TaskDone {
				return warnings, fmt.Errorf("cannot %s completed task %q", t.Action, t.ID)
			}

			// Warning: "keep" on cancelled task is a no-op but odd
			if t.Action == "keep" && existing.Status == state.TaskCancelled {
				warnings = append(warnings, fmt.Sprintf("task %q is cancelled — \"keep\" is a no-op", t.ID))
			}

		case "add":
			// Check dependencies for warnings
			for _, dep := range t.DependsOn {
				if existing, ok := taskMap[dep]; ok {
					if existing.Status == state.TaskFailed || existing.Status == state.TaskCancelled {
						warnings = append(warnings, fmt.Sprintf("new task %q depends on %s task %q", t.Title, existing.Status, dep))
					}
				}
			}

		default:
			return warnings, fmt.Errorf("unknown action %q for task %q", t.Action, t.ID)
		}
	}

	return warnings, nil
}

// MergeConversationHistory combines existing history with new messages,
// ensuring the total does not exceed maxMessages. When trimming, older
// messages are removed from the beginning.
func MergeConversationHistory(existing []state.ConversationMsg, newMsgs []state.ConversationMsg, maxMessages int) []state.ConversationMsg {
	combined := make([]state.ConversationMsg, 0, len(existing)+len(newMsgs))
	combined = append(combined, existing...)
	combined = append(combined, newMsgs...)

	if len(combined) <= maxMessages {
		return combined
	}

	// Trim from the beginning, keeping the last maxMessages-1 messages
	// and inserting a summary message at index 0
	trimCount := len(combined) - (maxMessages - 1)
	result := make([]state.ConversationMsg, 0, maxMessages)
	result = append(result, state.ConversationMsg{
		Role:    "system",
		Content: fmt.Sprintf("[Earlier conversation truncated — %d messages removed]", trimCount),
	})
	result = append(result, combined[trimCount:]...)
	return result
}

// SummarizeCompletedWork generates a brief summary of completed tasks.
func SummarizeCompletedWork(tasks []state.Task) string {
	var b strings.Builder
	for _, t := range tasks {
		if t.Status == state.TaskDone {
			fmt.Fprintf(&b, "%s: %s\n", t.ID, t.Title)
		}
	}
	return b.String()
}

// SummarizePendingWork generates a brief summary of pending tasks.
func SummarizePendingWork(tasks []state.Task) string {
	var b strings.Builder
	for _, t := range tasks {
		if t.Status == state.TaskPending {
			fmt.Fprintf(&b, "%s: %s\n", t.ID, t.Title)
		}
	}
	return b.String()
}

// SummarizeFailedWork generates a summary of failed tasks with retry context.
func SummarizeFailedWork(tasks []state.Task) string {
	var b strings.Builder
	for _, t := range tasks {
		if t.Status == state.TaskFailed {
			fmt.Fprintf(&b, "%s: %s (failed after %d retries)\n", t.ID, t.Title, t.Retries)
		}
	}
	return b.String()
}
