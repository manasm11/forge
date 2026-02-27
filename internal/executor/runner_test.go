package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/manasm11/forge/internal/state"
)

// ============================================================
// Task Ordering
// ============================================================

func TestRun_ExecutesTasksInDependencyOrder(t *testing.T) {
	t.Parallel()
	s := testState(
		mkTask("task-001", "Init", state.TaskPending, nil),
		mkTask("task-002", "Auth", state.TaskPending, []string{"task-001"}),
		mkTask("task-003", "API", state.TaskPending, []string{"task-001"}),
		mkTask("task-004", "Tests", state.TaskPending, []string{"task-002", "task-003"}),
	)

	git := NewMockGitOps()
	claude := NewMockClaudeExecutor(
		&ExecuteResult{Text: "done"},
		&ExecuteResult{Text: "done"},
		&ExecuteResult{Text: "done"},
		&ExecuteResult{Text: "done"},
	)
	tests := NewMockTestRunner(
		&TestResult{Passed: true},
		&TestResult{Passed: true},
		&TestResult{Passed: true},
		&TestResult{Passed: true},
	)

	var executionOrder []string
	var mu sync.Mutex
	onEvent := func(e TaskEvent) {
		if e.Type == EventTaskStart {
			mu.Lock()
			executionOrder = append(executionOrder, e.TaskID)
			mu.Unlock()
		}
	}

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tests, Claude: claude,
		OnEvent: onEvent, ContextFile: "ctx",
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	idx := make(map[string]int)
	for i, id := range executionOrder {
		idx[id] = i
	}
	if idx["task-001"] >= idx["task-002"] {
		t.Error("task-001 should execute before task-002")
	}
	if idx["task-001"] >= idx["task-003"] {
		t.Error("task-001 should execute before task-003")
	}
	if idx["task-004"] <= idx["task-002"] || idx["task-004"] <= idx["task-003"] {
		t.Error("task-004 should execute after task-002 and task-003")
	}
}

// ============================================================
// Successful Task Execution
// ============================================================

func TestRunTask_Success(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BranchPattern: "forge/{id}",
		MaxRetries:    3,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	git := NewMockGitOps()
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "implemented"})
	tr := NewMockTestRunner(&TestResult{Passed: true, Output: "PASS"})

	var events []TaskEvent
	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) { events = append(events, e) },
		ContextFile: "project context",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskDone {
		t.Errorf("status = %q, want done", outcome.Status)
	}
	if outcome.SHA == "" {
		t.Error("SHA should be set on success")
	}
	if outcome.Error != "" {
		t.Errorf("unexpected error: %q", outcome.Error)
	}

	if len(git.CreateBranchCalls) != 1 {
		t.Errorf("CreateBranch calls = %d", len(git.CreateBranchCalls))
	}
	if git.StageAllCalls != 1 {
		t.Errorf("StageAll calls = %d", git.StageAllCalls)
	}
	if len(git.CommitCalls) != 1 {
		t.Errorf("Commit calls = %d", len(git.CommitCalls))
	}
	if git.PushCalls != 1 {
		t.Errorf("Push calls = %d", git.PushCalls)
	}

	if len(claude.Calls) != 1 {
		t.Fatalf("Claude calls = %d", len(claude.Calls))
	}

	if len(tr.Calls) < 1 {
		t.Error("test command should have been run")
	}

	hasEvent := func(typ TaskEventType) bool {
		for _, e := range events {
			if e.Type == typ {
				return true
			}
		}
		return false
	}
	for _, evt := range []TaskEventType{
		EventTaskStart, EventBranchCreated, EventClaudeStart,
		EventClaudeDone, EventTestPassed, EventCommit, EventPush,
	} {
		if !hasEvent(evt) {
			t.Errorf("missing event type %d", evt)
		}
	}
}

// ============================================================
// Test Failure with Retry
// ============================================================

func TestRunTask_RetryOnTestFailure(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BranchPattern: "forge/{id}",
		MaxRetries:    2,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	git := NewMockGitOps()
	claude := NewMockClaudeExecutor(
		&ExecuteResult{Text: "initial implementation"},
		&ExecuteResult{Text: "fixed the bug"},
		&ExecuteResult{Text: "fixed again"},
	)
	tr := NewMockTestRunner(
		&TestResult{Passed: false, Output: "FAIL TestAuth"},
		&TestResult{Passed: false, Output: "FAIL TestAuth2"},
		&TestResult{Passed: true, Output: "PASS"},
	)

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskDone {
		t.Errorf("status = %q, want done (should succeed after retries)", outcome.Status)
	}
	if outcome.Retries != 2 {
		t.Errorf("retries = %d, want 2", outcome.Retries)
	}
	if len(claude.Calls) != 3 {
		t.Errorf("claude calls = %d, want 3", len(claude.Calls))
	}
	if len(claude.Calls) >= 2 && !strings.Contains(claude.Calls[1].Prompt, "FAIL TestAuth") {
		t.Error("retry prompt should contain previous test failure output")
	}
}

func TestRunTask_ExhaustsRetries(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BranchPattern: "forge/{id}",
		MaxRetries:    1,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	git := NewMockGitOps()
	claude := NewMockClaudeExecutor(
		&ExecuteResult{Text: "v1"}, &ExecuteResult{Text: "v2"},
	)
	tr := NewMockTestRunner(
		&TestResult{Passed: false, Output: "FAIL"},
		&TestResult{Passed: false, Output: "STILL FAIL"},
	)

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", outcome.Status)
	}
	if outcome.Error == "" {
		t.Error("should have error message")
	}
	if len(git.CommitCalls) > 0 {
		t.Error("should not commit on failure")
	}
	if git.PushCalls > 0 {
		t.Error("should not push on failure")
	}
}

func TestRunTask_ZeroRetries(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BranchPattern: "forge/{id}",
		MaxRetries:    0,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	git := NewMockGitOps()
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: false, Output: "FAIL"})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", outcome.Status)
	}
	if outcome.Retries != 0 {
		t.Errorf("retries = %d, want 0", outcome.Retries)
	}
}

// ============================================================
// Skip Tasks with Failed/Cancelled Dependencies
// ============================================================

func TestRun_SkipsTaskWithFailedDependency(t *testing.T) {
	t.Parallel()
	s := testState(
		mkTask("task-001", "Init", state.TaskFailed, nil),
		mkTask("task-002", "Auth", state.TaskPending, []string{"task-001"}),
		mkTask("task-003", "Standalone", state.TaskPending, nil),
	)
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.Run(context.Background())

	task2 := s.FindTask("task-002")
	if task2.Status != state.TaskSkipped {
		t.Errorf("task-002 status = %q, want skipped", task2.Status)
	}

	task3 := s.FindTask("task-003")
	if task3.Status != state.TaskDone {
		t.Errorf("task-003 status = %q, want done", task3.Status)
	}

	if len(claude.Calls) != 1 {
		t.Errorf("claude calls = %d, want 1", len(claude.Calls))
	}
}

func TestRun_SkipsTaskWithCancelledDependency(t *testing.T) {
	t.Parallel()
	s := testState(
		mkTask("task-001", "Cancelled", state.TaskCancelled, nil),
		mkTask("task-002", "Depends", state.TaskPending, []string{"task-001"}),
	)
	s.Settings = defaultSettings()

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: NewMockTestRunner(), Claude: NewMockClaudeExecutor(),
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.Run(context.Background())

	task2 := s.FindTask("task-002")
	if task2.Status != state.TaskSkipped {
		t.Errorf("status = %q, want skipped", task2.Status)
	}
}

// ============================================================
// Already Done Tasks Are Skipped
// ============================================================

func TestRun_SkipsAlreadyDoneTasks(t *testing.T) {
	t.Parallel()
	s := testState(
		mkTask("task-001", "Done", state.TaskDone, nil),
		mkTask("task-002", "Pending", state.TaskPending, []string{"task-001"}),
	)
	s.Settings = defaultSettings()

	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.Run(context.Background())

	if len(claude.Calls) != 1 {
		t.Errorf("claude calls = %d, want 1", len(claude.Calls))
	}
}

// ============================================================
// Cascading Failure
// ============================================================

func TestRun_CascadingFailureSkipsDependents(t *testing.T) {
	t.Parallel()
	s := testState(
		mkTask("task-001", "Init", state.TaskPending, nil),
		mkTask("task-002", "Build on 1", state.TaskPending, []string{"task-001"}),
		mkTask("task-003", "Build on 2", state.TaskPending, []string{"task-002"}),
		mkTask("task-004", "Independent", state.TaskPending, nil),
	)
	s.Settings = &state.Settings{
		TestCommand:   "test",
		BranchPattern: "forge/{id}",
		MaxRetries:    0,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	claude := NewMockClaudeExecutor(
		&ExecuteResult{Text: "impl1"},
		&ExecuteResult{Text: "impl4"},
	)
	tr := NewMockTestRunner(
		&TestResult{Passed: false, Output: "FAIL"},
		&TestResult{Passed: true},
	)

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.Run(context.Background())

	if s.FindTask("task-001").Status != state.TaskFailed {
		t.Error("task-001 should be failed")
	}
	if s.FindTask("task-002").Status != state.TaskSkipped {
		t.Error("task-002 should be skipped (depends on failed task-001)")
	}
	if s.FindTask("task-003").Status != state.TaskSkipped {
		t.Error("task-003 should be skipped (depends on skipped task-002)")
	}
	if s.FindTask("task-004").Status != state.TaskDone {
		t.Error("task-004 should be done (independent)")
	}
}

// ============================================================
// Git Error Handling
// ============================================================

func TestRunTask_GitCreateBranchFails(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.CreateBranchErr = fmt.Errorf("branch already exists")

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: NewMockTestRunner(), Claude: NewMockClaudeExecutor(),
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", outcome.Status)
	}
	if !strings.Contains(outcome.Error, "branch") {
		t.Errorf("error should mention branch: %q", outcome.Error)
	}
}

func TestRunTask_GitPushFails(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.PushErr = fmt.Errorf("remote rejected")
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", outcome.Status)
	}
}

func TestRunTask_CommitFails(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.CommitErr = fmt.Errorf("nothing to commit")
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", outcome.Status)
	}
}

// ============================================================
// Claude Error Handling
// ============================================================

func TestRunTask_ClaudeExecutionFails(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	claude := &MockClaudeExecutor{
		Results: []*ExecuteResult{nil},
		Errors:  []error{fmt.Errorf("claude CLI not found")},
	}

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: NewMockTestRunner(), Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed", outcome.Status)
	}
	if !strings.Contains(outcome.Error, "claude") {
		t.Errorf("error should mention claude: %q", outcome.Error)
	}
}

// ============================================================
// Context Cancellation
// ============================================================

func TestRun_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	s := testState(
		mkTask("task-001", "T1", state.TaskPending, nil),
		mkTask("task-002", "T2", state.TaskPending, nil),
		mkTask("task-003", "T3", state.TaskPending, nil),
	)
	s.Settings = defaultSettings()

	slowClaude := &slowMockClaude{delay: 100 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())

	var completedCount int
	var mu sync.Mutex
	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: NewMockTestRunner(&TestResult{Passed: true}),
		Claude: slowClaude,
		OnEvent: func(e TaskEvent) {
			if e.Type == EventTaskDone {
				mu.Lock()
				completedCount++
				if completedCount >= 1 {
					cancel()
				}
				mu.Unlock()
			}
		},
		ContextFile: "ctx",
	})

	err := runner.Run(ctx)

	if err == nil {
		t.Error("expected context cancellation error")
	}
	mu.Lock()
	if completedCount >= 3 {
		t.Error("should not have completed all tasks after cancellation")
	}
	mu.Unlock()
}

type slowMockClaude struct {
	delay time.Duration
}

func (s *slowMockClaude) Execute(ctx context.Context, opts ExecuteOpts) (*ExecuteResult, error) {
	select {
	case <-time.After(s.delay):
		return &ExecuteResult{Text: "done"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ============================================================
// No Changes After Claude (nothing to commit)
// ============================================================

func TestRunTask_NoChangesAfterClaude(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.HasStagedResult = false
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskFailed {
		t.Errorf("status = %q, want failed (no changes produced)", outcome.Status)
	}
	if len(git.CommitCalls) > 0 {
		t.Error("should not attempt commit with no staged changes")
	}
}

// ============================================================
// Build Command Execution
// ============================================================

func TestRunTask_RunsBuildCommandIfSet(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BuildCommand:  "go build ./...",
		BranchPattern: "forge/{id}",
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(
		&TestResult{Passed: true},
		&TestResult{Passed: true},
	)

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.RunTask(context.Background(), &s.Tasks[0])

	hasTest := false
	hasBuild := false
	for _, cmd := range tr.Calls {
		if cmd == "go test ./..." {
			hasTest = true
		}
		if cmd == "go build ./..." {
			hasBuild = true
		}
	}
	if !hasTest {
		t.Error("test command should have been run")
	}
	if !hasBuild {
		t.Error("build command should have been run")
	}
}

func TestRunTask_SkipsBuildIfEmpty(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BuildCommand:  "",
		BranchPattern: "forge/{id}",
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.RunTask(context.Background(), &s.Tasks[0])

	if len(tr.Calls) != 1 {
		t.Errorf("should only run test command, got %d calls: %v", len(tr.Calls), tr.Calls)
	}
}

func TestRunTask_BuildFailureTriggersRetry(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = &state.Settings{
		TestCommand:   "go test ./...",
		BuildCommand:  "go build ./...",
		BranchPattern: "forge/{id}",
		MaxRetries:    1,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}

	claude := NewMockClaudeExecutor(
		&ExecuteResult{Text: "v1"},
		&ExecuteResult{Text: "v2"},
	)
	tr := NewMockTestRunner(
		&TestResult{Passed: true},                        // test pass (attempt 1)
		&TestResult{Passed: false, Output: "build error"}, // build fail (attempt 1)
		&TestResult{Passed: true},                        // test pass (attempt 2)
		&TestResult{Passed: true},                        // build pass (attempt 2)
	)

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Status != state.TaskDone {
		t.Errorf("status = %q, want done", outcome.Status)
	}
}

// ============================================================
// State Persistence
// ============================================================

func TestRun_UpdatesStateAfterEachTask(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := testState(
		mkTask("task-001", "T1", state.TaskPending, nil),
		mkTask("task-002", "T2", state.TaskPending, nil),
	)
	s.Settings = defaultSettings()

	state.Save(dir, s)

	claude := NewMockClaudeExecutor(
		&ExecuteResult{Text: "done"},
		&ExecuteResult{Text: "done"},
	)
	tr := NewMockTestRunner(
		&TestResult{Passed: true},
		&TestResult{Passed: true},
	)

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: dir,
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.Run(context.Background())

	loaded, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if loaded.FindTask("task-001").Status != state.TaskDone {
		t.Error("task-001 should be persisted as done")
	}
	if loaded.FindTask("task-002").Status != state.TaskDone {
		t.Error("task-002 should be persisted as done")
	}
}

func TestRunTask_SetsTaskBranchAndSHA(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.CommitSHA = "deadbeef123456"
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.RunTask(context.Background(), &s.Tasks[0])

	if s.Tasks[0].Branch == "" {
		t.Error("task branch should be set")
	}
	if s.Tasks[0].GitSHA != "deadbeef123456" {
		t.Errorf("GitSHA = %q, want deadbeef123456", s.Tasks[0].GitSHA)
	}
}

// ============================================================
// Logging
// ============================================================

func TestRunTask_WritesLogFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "implemented the thing"})
	tr := NewMockTestRunner(&TestResult{Passed: true, Output: "PASS ok"})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: dir,
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if outcome.Logs == "" {
		t.Error("logs should not be empty")
	}
	if !strings.Contains(outcome.Logs, "implemented the thing") {
		t.Error("logs should contain claude output")
	}
	if !strings.Contains(outcome.Logs, "PASS ok") {
		t.Error("logs should contain test output")
	}
}

// ============================================================
// Resume (pick up from where we left off)
// ============================================================

func TestRun_ResumesFromPartialExecution(t *testing.T) {
	t.Parallel()
	now := time.Now()
	s := testState(
		mkTask("task-001", "Done already", state.TaskDone, nil),
		mkTask("task-002", "Was failed", state.TaskFailed, nil),
		mkTask("task-003", "Pending", state.TaskPending, []string{"task-001"}),
	)
	s.Tasks[0].CompletedAt = &now
	s.Settings = defaultSettings()

	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	var started []string
	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {
			if e.Type == EventTaskStart {
				started = append(started, e.TaskID)
			}
		},
		ContextFile: "ctx",
	})

	runner.Run(context.Background())

	for _, id := range started {
		if id == "task-001" {
			t.Error("should not re-execute already done task-001")
		}
	}
	if s.FindTask("task-003").Status != state.TaskDone {
		t.Error("task-003 should be done")
	}
}

// ============================================================
// Branch Already Exists (resume mid-task)
// ============================================================

func TestRunTask_ResumeOnExistingBranch(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.BranchExistsResult["forge/task-001"] = true
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	outcome := runner.RunTask(context.Background(), &s.Tasks[0])

	if len(git.CreateBranchCalls) > 0 {
		t.Error("should not create branch when it already exists")
	}
	if len(git.CheckoutCalls) < 1 || git.CheckoutCalls[0] != "forge/task-001" {
		t.Errorf("should checkout existing branch, got: %v", git.CheckoutCalls)
	}
	if outcome.Status != state.TaskDone {
		t.Errorf("status = %q, want done", outcome.Status)
	}
}

// ============================================================
// Checkout back to base branch after task
// ============================================================

func TestRunTask_ReturnsToBaseBranch(t *testing.T) {
	t.Parallel()
	s := testState(mkTask("task-001", "Init", state.TaskPending, nil))
	s.Settings = defaultSettings()

	git := NewMockGitOps()
	git.CurrentBranchResult = "main"
	claude := NewMockClaudeExecutor(&ExecuteResult{Text: "done"})
	tr := NewMockTestRunner(&TestResult{Passed: true})

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: git, Tests: tr, Claude: claude,
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	runner.RunTask(context.Background(), &s.Tasks[0])

	if len(git.CheckoutCalls) == 0 {
		t.Fatal("should have checkout calls")
	}
	lastCheckout := git.CheckoutCalls[len(git.CheckoutCalls)-1]
	if lastCheckout != "main" {
		t.Errorf("should return to base branch 'main', last checkout = %q", lastCheckout)
	}
}

// ============================================================
// Empty task list
// ============================================================

func TestRun_EmptyTaskList(t *testing.T) {
	t.Parallel()
	s := &state.State{Tasks: nil, Settings: defaultSettings()}

	runner := NewRunner(RunnerConfig{
		State: s, StateRoot: t.TempDir(),
		Git: NewMockGitOps(), Tests: NewMockTestRunner(),
		Claude: NewMockClaudeExecutor(),
		OnEvent: func(e TaskEvent) {}, ContextFile: "ctx",
	})

	err := runner.Run(context.Background())
	if err != nil {
		t.Errorf("empty task list should not error: %v", err)
	}
}

// ============================================================
// Test helpers
// ============================================================

func testState(tasks ...state.Task) *state.State {
	return &state.State{
		ProjectName: "test-project",
		PlanVersion: 1,
		Tasks:       tasks,
		Settings:    defaultSettings(),
	}
}

func mkTask(id, title string, status state.TaskStatus, deps []string) state.Task {
	return state.Task{
		ID:                 id,
		Title:              title,
		Status:             status,
		Description:        title + " description",
		AcceptanceCriteria: []string{title + " works"},
		Complexity:         "small",
		DependsOn:          deps,
		PlanVersionCreated: 1,
	}
}

func defaultSettings() *state.Settings {
	return &state.Settings{
		TestCommand:   "go test ./...",
		BranchPattern: "forge/{id}",
		MaxRetries:    2,
		MaxTurns:      state.MaxTurnsConfig{Small: 20, Medium: 35, Large: 50},
	}
}
