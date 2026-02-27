package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/manasm11/forge/internal/executor"
	"github.com/manasm11/forge/internal/state"
)

// ============================================================
// BuildTaskProgressList
// ============================================================

func TestBuildTaskProgressList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		tasks     []state.Task
		wantCount int
	}{
		{
			name:      "empty",
			tasks:     nil,
			wantCount: 0,
		},
		{
			name: "filters out cancelled",
			tasks: []state.Task{
				{ID: "task-001", Title: "A", Status: state.TaskPending, Complexity: "small"},
				{ID: "task-002", Title: "B", Status: state.TaskCancelled, Complexity: "small"},
				{ID: "task-003", Title: "C", Status: state.TaskPending, Complexity: "medium"},
			},
			wantCount: 2,
		},
		{
			name: "preserves order and sets max attempts",
			tasks: []state.Task{
				{ID: "task-001", Title: "A", Status: state.TaskDone, Complexity: "small"},
				{ID: "task-002", Title: "B", Status: state.TaskPending, Complexity: "large"},
			},
			wantCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			settings := &state.Settings{MaxRetries: 2, MaxTurns: state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50}}
			list := BuildTaskProgressList(tt.tasks, settings)
			if len(list) != tt.wantCount {
				t.Errorf("count = %d, want %d", len(list), tt.wantCount)
			}
		})
	}
}

func TestBuildTaskProgressList_MaxAttempts(t *testing.T) {
	t.Parallel()
	tasks := []state.Task{
		{ID: "task-001", Title: "A", Status: state.TaskPending, Complexity: "small"},
	}
	settings := &state.Settings{MaxRetries: 3}
	list := BuildTaskProgressList(tasks, settings)

	if list[0].MaxAttempts != 4 { // 1 initial + 3 retries
		t.Errorf("MaxAttempts = %d, want 4", list[0].MaxAttempts)
	}
}

func TestBuildTaskProgressList_DoneTasksPreserveTimestamps(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tasks := []state.Task{
		{ID: "task-001", Title: "A", Status: state.TaskDone, Complexity: "small",
			CompletedAt: &now, Retries: 1},
	}
	settings := &state.Settings{MaxRetries: 2}
	list := BuildTaskProgressList(tasks, settings)

	if list[0].FinishedAt == nil {
		t.Error("done task should have FinishedAt set")
	}
	if list[0].RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", list[0].RetryCount)
	}
}

// ============================================================
// ComputeExecutionStatus
// ============================================================

func TestComputeExecutionStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		tasks []state.Task
		want  ExecutionStatus
	}{
		{
			name: "all done",
			tasks: []state.Task{
				{Status: state.TaskDone},
				{Status: state.TaskDone},
			},
			want: ExecComplete,
		},
		{
			name: "has pending — still running",
			tasks: []state.Task{
				{Status: state.TaskDone},
				{Status: state.TaskPending},
			},
			want: ExecRunning,
		},
		{
			name: "has in-progress — still running",
			tasks: []state.Task{
				{Status: state.TaskDone},
				{Status: state.TaskInProgress},
			},
			want: ExecRunning,
		},
		{
			name: "failed with no pending — stopped",
			tasks: []state.Task{
				{Status: state.TaskDone},
				{Status: state.TaskFailed},
				{Status: state.TaskSkipped},
			},
			want: ExecStopped,
		},
		{
			name: "all skipped — stopped",
			tasks: []state.Task{
				{Status: state.TaskSkipped},
				{Status: state.TaskSkipped},
			},
			want: ExecStopped,
		},
		{
			name: "mixed: failed but still has runnable pending — running",
			tasks: []state.Task{
				{Status: state.TaskFailed},
				{Status: state.TaskPending}, // independent, can still run
			},
			want: ExecRunning,
		},
		{
			name:  "empty tasks",
			tasks: nil,
			want:  ExecComplete,
		},
		{
			name: "all cancelled",
			tasks: []state.Task{
				{Status: state.TaskCancelled},
			},
			want: ExecComplete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeExecutionStatus(tt.tasks)
			if got != tt.want {
				t.Errorf("ComputeExecutionStatus() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ============================================================
// ComputeExecutionSummary
// ============================================================

func TestComputeExecutionSummary(t *testing.T) {
	t.Parallel()
	now := time.Now()
	earlier := now.Add(-5 * time.Minute)
	progress := []TaskProgress{
		{TaskID: "task-001", Status: state.TaskDone, StartedAt: &earlier, FinishedAt: &now, RetryCount: 0},
		{TaskID: "task-002", Status: state.TaskDone, StartedAt: &earlier, FinishedAt: &now, RetryCount: 2},
		{TaskID: "task-003", Status: state.TaskFailed, StartedAt: &earlier, FinishedAt: &now, RetryCount: 3},
		{TaskID: "task-004", Status: state.TaskSkipped},
	}

	summary := ComputeExecutionSummary(progress)

	if summary.TotalTasks != 4 {
		t.Errorf("TotalTasks = %d", summary.TotalTasks)
	}
	if summary.Completed != 2 {
		t.Errorf("Completed = %d", summary.Completed)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed = %d", summary.Failed)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d", summary.Skipped)
	}
	if summary.TotalRetries != 5 {
		t.Errorf("TotalRetries = %d, want 5", summary.TotalRetries)
	}
}

func TestComputeExecutionSummary_Empty(t *testing.T) {
	t.Parallel()
	summary := ComputeExecutionSummary(nil)
	if summary.TotalTasks != 0 {
		t.Errorf("TotalTasks = %d", summary.TotalTasks)
	}
}

// ============================================================
// FormatProgressBar
// ============================================================

func TestFormatProgressBar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		done  int
		total int
		width int
		check func(string) bool
	}{
		{
			name: "0%", done: 0, total: 5, width: 20,
			check: func(s string) bool { return strings.Contains(s, "0/5") && strings.Contains(s, "0%") },
		},
		{
			name: "50%", done: 3, total: 6, width: 20,
			check: func(s string) bool { return strings.Contains(s, "3/6") && strings.Contains(s, "50%") },
		},
		{
			name: "100%", done: 5, total: 5, width: 20,
			check: func(s string) bool { return strings.Contains(s, "5/5") && strings.Contains(s, "100%") },
		},
		{
			name: "zero total", done: 0, total: 0, width: 20,
			check: func(s string) bool { return strings.Contains(s, "0/0") },
		},
		{
			name: "bar has filled blocks", done: 3, total: 10, width: 20,
			check: func(s string) bool { return strings.Contains(s, "█") && strings.Contains(s, "░") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FormatProgressBar(tt.done, tt.total, tt.width)
			if !tt.check(result) {
				t.Errorf("FormatProgressBar(%d, %d, %d) = %q", tt.done, tt.total, tt.width, result)
			}
		})
	}
}

// ============================================================
// FormatElapsed
// ============================================================

func TestFormatElapsed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{0, "0:00"},
		{12 * time.Second, "0:12"},
		{65 * time.Second, "1:05"},
		{3661 * time.Second, "1:01:01"},
		{5*time.Minute + 30*time.Second, "5:30"},
		{59 * time.Second, "0:59"},
		{60 * time.Second, "1:00"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := FormatElapsed(tt.dur)
			if got != tt.want {
				t.Errorf("FormatElapsed(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

// ============================================================
// EventToLogLine
// ============================================================

func TestEventToLogLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		event    executor.TaskEvent
		wantNil  bool
		wantType LogLineType
		wantText string
	}{
		{
			name:     "branch created",
			event:    executor.TaskEvent{Type: executor.EventBranchCreated, Message: "forge/task-001"},
			wantType: LogInfo,
			wantText: "Creating branch forge/task-001",
		},
		{
			name:     "claude start",
			event:    executor.TaskEvent{Type: executor.EventClaudeStart},
			wantType: LogInfo,
			wantText: "Running Claude Code",
		},
		{
			name:     "claude chunk",
			event:    executor.TaskEvent{Type: executor.EventClaudeChunk, Detail: "Writing auth.go..."},
			wantType: LogClaudeChunk,
			wantText: "Writing auth.go...",
		},
		{
			name:     "test passed",
			event:    executor.TaskEvent{Type: executor.EventTestPassed},
			wantType: LogSuccess,
			wantText: "Tests passed",
		},
		{
			name:     "test failed",
			event:    executor.TaskEvent{Type: executor.EventTestFailed, Detail: "FAIL TestAuth"},
			wantType: LogError,
		},
		{
			name:     "retry",
			event:    executor.TaskEvent{Type: executor.EventRetry, Message: "Retry 1/3"},
			wantType: LogWarning,
		},
		{
			name:     "commit",
			event:    executor.TaskEvent{Type: executor.EventCommit, Message: "abc123"},
			wantType: LogSuccess,
		},
		{
			name:     "push",
			event:    executor.TaskEvent{Type: executor.EventPush},
			wantType: LogSuccess,
		},
		{
			name:     "task done",
			event:    executor.TaskEvent{Type: executor.EventTaskDone},
			wantType: LogSuccess,
		},
		{
			name:     "task failed",
			event:    executor.TaskEvent{Type: executor.EventTaskFailed, Message: "exhausted retries"},
			wantType: LogError,
		},
		{
			name:     "task skipped",
			event:    executor.TaskEvent{Type: executor.EventTaskSkipped, Message: "dependency failed"},
			wantType: LogWarning,
		},
		{
			name:     "build passed",
			event:    executor.TaskEvent{Type: executor.EventBuildPassed},
			wantType: LogSuccess,
		},
		{
			name:     "build failed",
			event:    executor.TaskEvent{Type: executor.EventBuildFailed, Detail: "compile error"},
			wantType: LogError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			line := EventToLogLine(tt.event)
			if tt.wantNil {
				if line != nil {
					t.Error("expected nil")
				}
				return
			}
			if line == nil {
				t.Fatal("expected non-nil LogLine")
			}
			if line.Type != tt.wantType {
				t.Errorf("Type = %d, want %d", line.Type, tt.wantType)
			}
			if tt.wantText != "" && !strings.Contains(line.Text, tt.wantText) {
				t.Errorf("Text = %q, want containing %q", line.Text, tt.wantText)
			}
		})
	}
}

// ============================================================
// TasksRemaining
// ============================================================

func TestTasksRemaining(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		tasks []state.Task
		want  int
	}{
		{
			name: "mixed statuses",
			tasks: []state.Task{
				{Status: state.TaskDone},
				{Status: state.TaskPending},
				{Status: state.TaskPending},
				{Status: state.TaskFailed},
				{Status: state.TaskSkipped},
				{Status: state.TaskInProgress},
			},
			want: 3, // 2 pending + 1 in-progress
		},
		{
			name: "all done",
			tasks: []state.Task{
				{Status: state.TaskDone},
				{Status: state.TaskDone},
			},
			want: 0,
		},
		{
			name:  "empty",
			tasks: nil,
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TasksRemaining(tt.tasks)
			if got != tt.want {
				t.Errorf("TasksRemaining() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ============================================================
// FormatTaskStatusLine
// ============================================================

func TestFormatTaskStatusLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		tp          TaskProgress
		selected    bool
		mustContain []string
	}{
		{
			name: "done task",
			tp: TaskProgress{
				TaskID: "task-001", Title: "Init", Complexity: "small",
				Status: state.TaskDone, Elapsed: 12 * time.Second,
			},
			mustContain: []string{"✅", "task-001", "small", "Init", "0:12"},
		},
		{
			name: "running task",
			tp: TaskProgress{
				TaskID: "task-002", Title: "Auth", Complexity: "medium",
				Status: state.TaskInProgress, Elapsed: 83 * time.Second, Attempt: 1, MaxAttempts: 3,
			},
			mustContain: []string{"🔄", "task-002", "Auth", "1:23"},
		},
		{
			name: "failed task with retries",
			tp: TaskProgress{
				TaskID: "task-003", Title: "Payment", Complexity: "large",
				Status: state.TaskFailed, RetryCount: 3,
			},
			mustContain: []string{"❌", "task-003", "Payment", "3 retries"},
		},
		{
			name: "skipped task",
			tp: TaskProgress{
				TaskID: "task-004", Title: "Depends on failed", Complexity: "small",
				Status: state.TaskSkipped,
			},
			mustContain: []string{"⏭", "task-004", "skipped"},
		},
		{
			name: "pending task",
			tp: TaskProgress{
				TaskID: "task-005", Title: "Upcoming", Complexity: "medium",
				Status: state.TaskPending,
			},
			mustContain: []string{"task-005", "Upcoming"},
		},
		{
			name: "selected task has arrow",
			tp: TaskProgress{
				TaskID: "task-003", Title: "Auth", Complexity: "medium",
				Status: state.TaskInProgress,
			},
			selected:    true,
			mustContain: []string{"→"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			line := FormatTaskStatusLine(tt.tp, tt.selected, 80)
			for _, s := range tt.mustContain {
				if !strings.Contains(line, s) {
					t.Errorf("line missing %q\ngot: %q", s, line)
				}
			}
		})
	}
}

// ============================================================
// FormatCompletionMessage
// ============================================================

func TestFormatCompletionMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		status      ExecutionStatus
		summary     ExecutionSummary
		mustContain []string
	}{
		{
			name:   "all complete",
			status: ExecComplete,
			summary: ExecutionSummary{
				TotalTasks: 5, Completed: 5, Failed: 0, Skipped: 0,
				TotalDuration: 11*time.Minute + 6*time.Second,
			},
			mustContain: []string{"Complete", "5/5"},
		},
		{
			name:   "stopped with failures",
			status: ExecStopped,
			summary: ExecutionSummary{
				TotalTasks: 5, Completed: 3, Failed: 1, Skipped: 1,
			},
			mustContain: []string{"Stopped", "3/5", "1 failed"},
		},
		{
			name:   "cancelled",
			status: ExecCancelled,
			summary: ExecutionSummary{
				TotalTasks: 5, Completed: 2,
			},
			mustContain: []string{"Cancelled", "2/5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := FormatCompletionMessage(tt.status, tt.summary)
			for _, s := range tt.mustContain {
				if !strings.Contains(msg, s) {
					t.Errorf("message missing %q\ngot: %q", s, msg)
				}
			}
		})
	}
}

// ============================================================
// FormatSummaryText
// ============================================================

func TestFormatSummaryText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		summary        ExecutionSummary
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "full success",
			summary: ExecutionSummary{
				TotalTasks: 3, Completed: 3, Failed: 0, Skipped: 0,
				TotalRetries: 0, TotalDuration: 5 * time.Minute,
				Branches: []string{"forge/task-001", "forge/task-002", "forge/task-003"},
			},
			mustContain:    []string{"3 tasks completed", "5:00"},
			mustNotContain: []string{"failed", "skipped", "retries"},
		},
		{
			name: "with failures and retries",
			summary: ExecutionSummary{
				TotalTasks: 5, Completed: 3, Failed: 1, Skipped: 1,
				TotalRetries: 4, TotalDuration: 10 * time.Minute,
			},
			mustContain: []string{"3 tasks completed", "1 failed", "1 skipped", "4 retries"},
		},
		{
			name: "no retries — omit retries line",
			summary: ExecutionSummary{
				TotalTasks: 2, Completed: 2, TotalRetries: 0,
				TotalDuration: 2 * time.Minute,
			},
			mustNotContain: []string{"retries"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text := FormatSummaryText(tt.summary)
			for _, s := range tt.mustContain {
				if !strings.Contains(text, s) {
					t.Errorf("text missing %q\ngot:\n%s", s, text)
				}
			}
			for _, s := range tt.mustNotContain {
				if strings.Contains(text, s) {
					t.Errorf("text should not contain %q\ngot:\n%s", s, text)
				}
			}
		})
	}
}

// ============================================================
// Integration: Event stream → progress update
// ============================================================

func TestEventStreamUpdatesProgress(t *testing.T) {
	t.Parallel()

	progress := []TaskProgress{
		{TaskID: "task-001", Title: "Init", Status: state.TaskPending, MaxAttempts: 3},
		{TaskID: "task-002", Title: "Auth", Status: state.TaskPending, MaxAttempts: 3},
	}

	// Simulate event stream
	events := []executor.TaskEvent{
		{TaskID: "task-001", Type: executor.EventTaskStart},
		{TaskID: "task-001", Type: executor.EventBranchCreated, Message: "forge/task-001"},
		{TaskID: "task-001", Type: executor.EventClaudeStart},
		{TaskID: "task-001", Type: executor.EventClaudeDone},
		{TaskID: "task-001", Type: executor.EventTestPassed},
		{TaskID: "task-001", Type: executor.EventCommit, Message: "abc123"},
		{TaskID: "task-001", Type: executor.EventPush},
		{TaskID: "task-001", Type: executor.EventTaskDone},
	}

	for _, e := range events {
		ApplyEventToProgress(progress, e)
	}

	if progress[0].Status != state.TaskDone {
		t.Errorf("task-001 status = %q, want done", progress[0].Status)
	}
	if len(progress[0].LogLines) == 0 {
		t.Error("task-001 should have log lines")
	}
	if progress[1].Status != state.TaskPending {
		t.Errorf("task-002 should still be pending, got %q", progress[1].Status)
	}
}

func TestEventStreamUpdatesProgress_Retry(t *testing.T) {
	t.Parallel()

	progress := []TaskProgress{
		{TaskID: "task-001", Title: "Init", Status: state.TaskPending, MaxAttempts: 3},
	}

	events := []executor.TaskEvent{
		{TaskID: "task-001", Type: executor.EventTaskStart},
		{TaskID: "task-001", Type: executor.EventClaudeStart},
		{TaskID: "task-001", Type: executor.EventClaudeDone},
		{TaskID: "task-001", Type: executor.EventTestFailed, Detail: "FAIL"},
		{TaskID: "task-001", Type: executor.EventRetry, Message: "Retry 1/2"},
		{TaskID: "task-001", Type: executor.EventClaudeStart},
		{TaskID: "task-001", Type: executor.EventClaudeDone},
		{TaskID: "task-001", Type: executor.EventTestPassed},
		{TaskID: "task-001", Type: executor.EventCommit, Message: "abc"},
		{TaskID: "task-001", Type: executor.EventPush},
		{TaskID: "task-001", Type: executor.EventTaskDone},
	}

	for _, e := range events {
		ApplyEventToProgress(progress, e)
	}

	if progress[0].Attempt != 2 {
		t.Errorf("Attempt = %d, want 2", progress[0].Attempt)
	}
	if progress[0].RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", progress[0].RetryCount)
	}
}

func TestEventStreamUpdatesProgress_Failure(t *testing.T) {
	t.Parallel()

	progress := []TaskProgress{
		{TaskID: "task-001", Title: "Init", Status: state.TaskPending, MaxAttempts: 2},
	}

	events := []executor.TaskEvent{
		{TaskID: "task-001", Type: executor.EventTaskStart},
		{TaskID: "task-001", Type: executor.EventTestFailed},
		{TaskID: "task-001", Type: executor.EventRetry},
		{TaskID: "task-001", Type: executor.EventTestFailed},
		{TaskID: "task-001", Type: executor.EventTaskFailed, Message: "exhausted"},
	}

	for _, e := range events {
		ApplyEventToProgress(progress, e)
	}

	if progress[0].Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", progress[0].Status)
	}
}

// ============================================================
// Log line limit
// ============================================================

func TestApplyEventToProgress_LimitsLogLines(t *testing.T) {
	t.Parallel()
	progress := []TaskProgress{
		{TaskID: "task-001", Status: state.TaskInProgress, MaxAttempts: 1},
	}

	// Send 200 claude chunks
	for i := 0; i < 200; i++ {
		ApplyEventToProgress(progress, executor.TaskEvent{
			TaskID: "task-001", Type: executor.EventClaudeChunk,
			Detail: "chunk",
		})
	}

	// Should cap at a reasonable limit (e.g. 100)
	if len(progress[0].LogLines) > 150 {
		t.Errorf("log lines = %d, should be capped", len(progress[0].LogLines))
	}
}
