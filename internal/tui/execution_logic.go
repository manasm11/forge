package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/manasm11/forge/internal/executor"
	"github.com/manasm11/forge/internal/state"
)

// ExecutionStatus represents the overall execution state.
type ExecutionStatus int

const (
	ExecRunning   ExecutionStatus = iota
	ExecPaused
	ExecComplete  // all tasks done
	ExecStopped   // some tasks failed/skipped, nothing left to run
	ExecCancelled // user quit mid-execution
)

// TaskProgress tracks live progress for a single task.
type TaskProgress struct {
	TaskID      string
	Title       string
	Complexity  string
	Status      state.TaskStatus
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Elapsed     time.Duration
	Attempt     int // current attempt (1-based)
	MaxAttempts int
	LogLines    []LogLine // streaming log entries
	RetryCount  int       // total retries used
}

// LogLine is a single line in the task's live log.
type LogLine struct {
	Text      string
	Type      LogLineType
	Timestamp time.Time
}

// LogLineType classifies log line severity/purpose.
type LogLineType int

const (
	LogInfo LogLineType = iota
	LogSuccess
	LogError
	LogWarning
	LogClaudeChunk
)

// ExecutionSummary is computed when execution finishes.
type ExecutionSummary struct {
	TotalTasks    int
	Completed     int
	Failed        int
	Skipped       int
	TotalRetries  int
	TotalDuration time.Duration
	Branches      []string
}

const maxLogLines = 100

// BuildTaskProgressList creates the initial progress list from state tasks.
// Cancelled tasks are filtered out. MaxAttempts = 1 + MaxRetries.
func BuildTaskProgressList(tasks []state.Task, settings *state.Settings) []TaskProgress {
	var result []TaskProgress
	maxRetries := 0
	if settings != nil {
		maxRetries = settings.MaxRetries
	}

	for _, t := range tasks {
		if t.Status == state.TaskCancelled {
			continue
		}
		tp := TaskProgress{
			TaskID:      t.ID,
			Title:       t.Title,
			Complexity:  t.Complexity,
			Status:      t.Status,
			MaxAttempts: 1 + maxRetries,
			RetryCount:  t.Retries,
		}
		if t.Status == state.TaskDone && t.CompletedAt != nil {
			fin := *t.CompletedAt
			tp.FinishedAt = &fin
		}
		result = append(result, tp)
	}
	return result
}

// ComputeExecutionStatus determines overall status from task states.
func ComputeExecutionStatus(tasks []state.Task) ExecutionStatus {
	hasPending := false
	hasInProgress := false
	hasFailed := false
	hasSkipped := false
	hasDone := false

	for _, t := range tasks {
		switch t.Status {
		case state.TaskPending:
			hasPending = true
		case state.TaskInProgress:
			hasInProgress = true
		case state.TaskFailed:
			hasFailed = true
		case state.TaskSkipped:
			hasSkipped = true
		case state.TaskDone:
			hasDone = true
		case state.TaskCancelled:
			// ignored
		}
	}

	if hasInProgress || hasPending {
		return ExecRunning
	}
	if hasFailed || hasSkipped {
		if !hasDone && !hasFailed && !hasSkipped {
			return ExecComplete
		}
		return ExecStopped
	}
	return ExecComplete
}

// ComputeExecutionSummary calculates the final summary.
func ComputeExecutionSummary(progress []TaskProgress) ExecutionSummary {
	s := ExecutionSummary{
		TotalTasks: len(progress),
	}

	var earliest *time.Time
	var latest *time.Time

	for _, tp := range progress {
		switch tp.Status {
		case state.TaskDone:
			s.Completed++
		case state.TaskFailed:
			s.Failed++
		case state.TaskSkipped:
			s.Skipped++
		}
		s.TotalRetries += tp.RetryCount

		if tp.StartedAt != nil {
			if earliest == nil || tp.StartedAt.Before(*earliest) {
				t := *tp.StartedAt
				earliest = &t
			}
		}
		if tp.FinishedAt != nil {
			if latest == nil || tp.FinishedAt.After(*latest) {
				t := *tp.FinishedAt
				latest = &t
			}
		}
	}

	if earliest != nil && latest != nil {
		s.TotalDuration = latest.Sub(*earliest)
	}

	return s
}

// FormatProgressBar produces a text progress bar: ████████░░░░░░ 3/7 (43%)
func FormatProgressBar(done, total, width int) string {
	if total == 0 {
		return "░ 0/0 (0%)"
	}
	pct := done * 100 / total
	filled := done * width / total
	empty := width - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("%s %d/%d (%d%%)", bar, done, total, pct)
}

// FormatElapsed formats a duration as M:SS or H:MM:SS.
func FormatElapsed(d time.Duration) string {
	total := int(d.Seconds())
	if total >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", total/3600, (total%3600)/60, total%60)
	}
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// FormatSummaryText produces the human-readable summary block.
func FormatSummaryText(summary ExecutionSummary) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%d tasks completed in %s", summary.Completed, FormatElapsed(summary.TotalDuration))

	if summary.Failed > 0 {
		fmt.Fprintf(&b, "\n%d failed", summary.Failed)
	}
	if summary.Skipped > 0 {
		fmt.Fprintf(&b, "\n%d skipped", summary.Skipped)
	}
	if summary.TotalRetries > 0 {
		fmt.Fprintf(&b, "\n%d retries across all tasks", summary.TotalRetries)
	}

	if len(summary.Branches) > 0 {
		fmt.Fprintf(&b, "\nBranches: %s", strings.Join(summary.Branches, ", "))
	}

	return b.String()
}

// EventToLogLine converts an executor.TaskEvent into a displayable LogLine.
func EventToLogLine(event executor.TaskEvent) *LogLine {
	ts := time.Now()
	if event.Timestamp > 0 {
		ts = time.UnixMilli(event.Timestamp)
	}

	switch event.Type {
	case executor.EventBranchCreated:
		return &LogLine{Text: "Creating branch " + event.Message, Type: LogInfo, Timestamp: ts}
	case executor.EventClaudeStart:
		return &LogLine{Text: "Running Claude Code", Type: LogInfo, Timestamp: ts}
	case executor.EventClaudeChunk:
		return &LogLine{Text: event.Detail, Type: LogClaudeChunk, Timestamp: ts}
	case executor.EventClaudeDone:
		return &LogLine{Text: "Claude Code finished", Type: LogInfo, Timestamp: ts}
	case executor.EventTestStart:
		text := "Running tests"
		if event.Message != "" {
			text += ": " + event.Message
		}
		return &LogLine{Text: text, Type: LogInfo, Timestamp: ts}
	case executor.EventTestPassed:
		return &LogLine{Text: "Tests passed", Type: LogSuccess, Timestamp: ts}
	case executor.EventTestFailed:
		text := "Tests failed"
		if event.Detail != "" {
			text += "\n" + event.Detail
		}
		return &LogLine{Text: text, Type: LogError, Timestamp: ts}
	case executor.EventBuildStart:
		text := "Running build"
		if event.Message != "" {
			text += ": " + event.Message
		}
		return &LogLine{Text: text, Type: LogInfo, Timestamp: ts}
	case executor.EventBuildPassed:
		return &LogLine{Text: "Build passed", Type: LogSuccess, Timestamp: ts}
	case executor.EventBuildFailed:
		text := "Build failed"
		if event.Detail != "" {
			text += "\n" + event.Detail
		}
		return &LogLine{Text: text, Type: LogError, Timestamp: ts}
	case executor.EventRetry:
		text := event.Message
		if text == "" {
			text = "Retrying..."
		}
		return &LogLine{Text: text, Type: LogWarning, Timestamp: ts}
	case executor.EventCommit:
		return &LogLine{Text: "Committed: " + event.Message, Type: LogSuccess, Timestamp: ts}
	case executor.EventPush:
		return &LogLine{Text: "Pushed to origin", Type: LogSuccess, Timestamp: ts}
	case executor.EventTaskDone:
		return &LogLine{Text: "Task complete", Type: LogSuccess, Timestamp: ts}
	case executor.EventTaskFailed:
		text := "Task failed"
		if event.Message != "" {
			text += ": " + event.Message
		}
		return &LogLine{Text: text, Type: LogError, Timestamp: ts}
	case executor.EventTaskSkipped:
		text := "Task skipped"
		if event.Message != "" {
			text += ": " + event.Message
		}
		return &LogLine{Text: text, Type: LogWarning, Timestamp: ts}
	case executor.EventError:
		return &LogLine{Text: "Error: " + event.Message, Type: LogError, Timestamp: ts}
	case executor.EventTaskStart:
		return &LogLine{Text: "Starting task: " + event.Message, Type: LogInfo, Timestamp: ts}
	default:
		return nil
	}
}

// TasksRemaining returns the count of tasks not yet done/failed/skipped/cancelled.
func TasksRemaining(tasks []state.Task) int {
	count := 0
	for _, t := range tasks {
		switch t.Status {
		case state.TaskPending, state.TaskInProgress:
			count++
		}
	}
	return count
}

// FormatTaskStatusLine renders a single task line for the list.
func FormatTaskStatusLine(tp TaskProgress, selected bool, width int) string {
	var icon string
	switch tp.Status {
	case state.TaskDone:
		icon = "✅"
	case state.TaskInProgress:
		icon = "🔄"
	case state.TaskFailed:
		icon = "❌"
	case state.TaskSkipped:
		icon = "⏭"
	default:
		icon = "  "
	}

	prefix := "  "
	if selected {
		prefix = "→ "
	}

	complexity := fmt.Sprintf("[%s]", tp.Complexity)

	var suffix string
	if tp.Status == state.TaskInProgress || tp.Status == state.TaskDone {
		suffix = " " + FormatElapsed(tp.Elapsed)
	}
	if tp.Status == state.TaskFailed && tp.RetryCount > 0 {
		suffix = fmt.Sprintf(" (%d retries)", tp.RetryCount)
	}
	if tp.Status == state.TaskSkipped {
		suffix = " skipped"
	}

	return fmt.Sprintf("%s%s %s %s %s%s", prefix, icon, tp.TaskID, complexity, tp.Title, suffix)
}

// FormatCompletionMessage returns the header message based on execution status.
func FormatCompletionMessage(status ExecutionStatus, summary ExecutionSummary) string {
	done := fmt.Sprintf("%d/%d", summary.Completed, summary.TotalTasks)

	switch status {
	case ExecComplete:
		return fmt.Sprintf("Execution Complete! %s tasks done", done)
	case ExecStopped:
		msg := fmt.Sprintf("Execution Stopped — %s tasks", done)
		if summary.Failed > 0 {
			msg += fmt.Sprintf(", %d failed", summary.Failed)
		}
		if summary.Skipped > 0 {
			msg += fmt.Sprintf(", %d skipped", summary.Skipped)
		}
		return msg
	case ExecCancelled:
		return fmt.Sprintf("Execution Cancelled — %s tasks completed", done)
	default:
		return fmt.Sprintf("Executing — %s tasks done", done)
	}
}

// ApplyEventToProgress updates the progress list with a task event.
func ApplyEventToProgress(progress []TaskProgress, event executor.TaskEvent) {
	// Find matching task
	var tp *TaskProgress
	for i := range progress {
		if progress[i].TaskID == event.TaskID {
			tp = &progress[i]
			break
		}
	}
	if tp == nil {
		return
	}

	// Update status based on event type
	switch event.Type {
	case executor.EventTaskStart:
		tp.Status = state.TaskInProgress
		now := time.Now()
		tp.StartedAt = &now
		tp.Attempt = 1
	case executor.EventRetry:
		tp.Attempt++
		tp.RetryCount++
	case executor.EventTaskDone:
		tp.Status = state.TaskDone
		now := time.Now()
		tp.FinishedAt = &now
		if tp.StartedAt != nil {
			tp.Elapsed = now.Sub(*tp.StartedAt)
		}
	case executor.EventTaskFailed:
		tp.Status = state.TaskFailed
		now := time.Now()
		tp.FinishedAt = &now
		if tp.StartedAt != nil {
			tp.Elapsed = now.Sub(*tp.StartedAt)
		}
	case executor.EventTaskSkipped:
		tp.Status = state.TaskSkipped
	}

	// Append log line
	if line := EventToLogLine(event); line != nil {
		tp.LogLines = append(tp.LogLines, *line)
		// Cap log lines to prevent memory issues
		if len(tp.LogLines) > maxLogLines {
			tp.LogLines = tp.LogLines[len(tp.LogLines)-maxLogLines:]
		}
	}
}
