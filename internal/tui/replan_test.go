package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/state"
)

// ============================================================
// BuildReplanContext
// ============================================================

func TestBuildReplanContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		state         *state.State
		wantCompleted int
		wantPending   int
		wantFailed    int
		wantVersion   int
	}{
		{
			name: "mixed task statuses",
			state: &state.State{
				PlanVersion: 2,
				ConversationHistory: []state.ConversationMsg{
					{Role: "user", Content: "hello"},
					{Role: "assistant", Content: "hi"},
				},
				Tasks: []state.Task{
					{ID: "task-001", Status: state.TaskDone},
					{ID: "task-002", Status: state.TaskDone},
					{ID: "task-003", Status: state.TaskPending},
					{ID: "task-004", Status: state.TaskFailed},
					{ID: "task-005", Status: state.TaskCancelled},
				},
			},
			wantCompleted: 2,
			wantPending:   1,
			wantFailed:    1,
			wantVersion:   2,
		},
		{
			name: "all done",
			state: &state.State{
				PlanVersion: 1,
				Tasks: []state.Task{
					{ID: "task-001", Status: state.TaskDone},
				},
			},
			wantCompleted: 1,
			wantPending:   0,
			wantFailed:    0,
			wantVersion:   1,
		},
		{
			name: "empty state",
			state: &state.State{
				PlanVersion: 0,
			},
			wantCompleted: 0,
			wantPending:   0,
			wantFailed:    0,
			wantVersion:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := BuildReplanContext(tt.state)
			if ctx.CompletedCount != tt.wantCompleted {
				t.Errorf("CompletedCount = %d, want %d", ctx.CompletedCount, tt.wantCompleted)
			}
			if ctx.PendingCount != tt.wantPending {
				t.Errorf("PendingCount = %d, want %d", ctx.PendingCount, tt.wantPending)
			}
			if ctx.FailedCount != tt.wantFailed {
				t.Errorf("FailedCount = %d, want %d", ctx.FailedCount, tt.wantFailed)
			}
			if ctx.PlanVersion != tt.wantVersion {
				t.Errorf("PlanVersion = %d, want %d", ctx.PlanVersion, tt.wantVersion)
			}
			if ctx.SystemContext == "" {
				t.Error("SystemContext should not be empty")
			}
		})
	}
}

// ============================================================
// BuildReplanSystemMessage
// ============================================================

func TestBuildReplanSystemMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		ctx            ReplanContext
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "shows completed and pending counts",
			ctx: ReplanContext{
				CompletedCount: 3,
				PendingCount:   4,
				FailedCount:    0,
				PlanVersion:    2,
			},
			mustContain:    []string{"3 completed", "4 pending", "Welcome back"},
			mustNotContain: []string{"failed"},
		},
		{
			name: "includes failed count when non-zero",
			ctx: ReplanContext{
				CompletedCount: 2,
				PendingCount:   3,
				FailedCount:    1,
				PlanVersion:    1,
			},
			mustContain: []string{"1 failed"},
		},
		{
			name: "zero completed",
			ctx: ReplanContext{
				CompletedCount: 0,
				PendingCount:   5,
			},
			mustContain: []string{"5 pending"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := BuildReplanSystemMessage(tt.ctx)
			for _, s := range tt.mustContain {
				if !strings.Contains(msg, s) {
					t.Errorf("message missing %q\ngot: %s", s, msg)
				}
			}
			for _, s := range tt.mustNotContain {
				if strings.Contains(msg, s) {
					t.Errorf("message should not contain %q\ngot: %s", s, msg)
				}
			}
		})
	}
}

// ============================================================
// BuildReplanPrompt
// ============================================================

func TestBuildReplanPrompt(t *testing.T) {
	t.Parallel()
	ctx := ReplanContext{
		PlanVersion:    2,
		CompletedCount: 2,
		PendingCount:   3,
		SystemContext:  "COMPLETED TASKS:\n- task-001: Init\n\nPENDING TASKS:\n- task-002: Auth",
	}

	prompt := BuildReplanPrompt(ctx)

	mustContain := []string{
		"COMPLETED TASKS",
		"task-001",
		"PENDING TASKS",
		"task-002",
		"<plan_update>",                      // mentions the expected output format
		"CANNOT modify or remove completed", // instruction to protect done tasks
	}
	for _, s := range mustContain {
		if !strings.Contains(prompt, s) {
			t.Errorf("prompt missing %q", s)
		}
	}
}

// ============================================================
// ValidatePlanUpdate
// ============================================================

func TestValidatePlanUpdate(t *testing.T) {
	t.Parallel()
	baseState := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Init", Status: state.TaskDone},
			{ID: "task-002", Title: "Auth", Status: state.TaskPending},
			{ID: "task-003", Title: "API", Status: state.TaskPending},
			{ID: "task-004", Title: "Deploy", Status: state.TaskFailed},
			{ID: "task-005", Title: "Old", Status: state.TaskCancelled},
		},
	}

	tests := []struct {
		name         string
		update       *claude.PlanUpdateJSON
		wantErr      bool
		wantWarnings int
	}{
		{
			name: "valid update — all action types",
			update: &claude.PlanUpdateJSON{
				Summary: "Updated plan",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-001", Action: "keep"},
					{ID: "task-002", Action: "modify", Title: "Updated auth"},
					{ID: "task-003", Action: "remove", Reason: "not needed"},
					{Action: "add", Title: "New task", Description: "do stuff",
						AcceptanceCriteria: []string{"works"}, Complexity: "small"},
				},
			},
			wantErr:      false,
			wantWarnings: 0,
		},
		{
			name: "error — modify completed task",
			update: &claude.PlanUpdateJSON{
				Summary: "Bad update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-001", Action: "modify", Title: "Changed done task"},
				},
			},
			wantErr: true,
		},
		{
			name: "error — remove completed task",
			update: &claude.PlanUpdateJSON{
				Summary: "Bad update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-001", Action: "remove", Reason: "oops"},
				},
			},
			wantErr: true,
		},
		{
			name: "error — reference nonexistent task",
			update: &claude.PlanUpdateJSON{
				Summary: "Bad update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-999", Action: "keep"},
				},
			},
			wantErr: true,
		},
		{
			name: "error — unknown action",
			update: &claude.PlanUpdateJSON{
				Summary: "Bad update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-002", Action: "yeet"},
				},
			},
			wantErr: true,
		},
		{
			name: "error — duplicate task IDs",
			update: &claude.PlanUpdateJSON{
				Summary: "Bad update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-002", Action: "keep"},
					{ID: "task-002", Action: "modify", Title: "changed"},
				},
			},
			wantErr: true,
		},
		{
			name: "warning — keep on cancelled task",
			update: &claude.PlanUpdateJSON{
				Summary: "Odd update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-005", Action: "keep"},
				},
			},
			wantErr:      false,
			wantWarnings: 1,
		},
		{
			name: "warning — add task depending on failed task",
			update: &claude.PlanUpdateJSON{
				Summary: "Risky update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{Action: "add", Title: "New", Description: "depends on failed",
						AcceptanceCriteria: []string{"x"}, Complexity: "small",
						DependsOn: []string{"task-004"}},
				},
			},
			wantErr:      false,
			wantWarnings: 1,
		},
		{
			name: "valid — keep on done and modify pending",
			update: &claude.PlanUpdateJSON{
				Summary: "Simple update",
				Tasks: []claude.PlanUpdateTaskJSON{
					{ID: "task-001", Action: "keep"},
					{ID: "task-002", Action: "modify", Title: "Better auth",
						Description: "OAuth2", AcceptanceCriteria: []string{"OAuth works"},
						Complexity: "medium"},
				},
			},
			wantErr:      false,
			wantWarnings: 0,
		},
		{
			name: "valid — add with dependency on completed task",
			update: &claude.PlanUpdateJSON{
				Summary: "Extend",
				Tasks: []claude.PlanUpdateTaskJSON{
					{Action: "add", Title: "Caching", Description: "Add redis",
						AcceptanceCriteria: []string{"cache works"}, Complexity: "medium",
						DependsOn: []string{"task-001"}},
				},
			},
			wantErr:      false,
			wantWarnings: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Deep copy state so parallel tests don't interfere
			s := copyState(baseState)
			warnings, err := ValidatePlanUpdate(s, tt.update)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("warnings count = %d, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}
		})
	}
}

// ============================================================
// MergeConversationHistory
// ============================================================

func TestMergeConversationHistory(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		existing    []state.ConversationMsg
		newMsgs     []state.ConversationMsg
		max         int
		wantCount   int
		wantLastMsg string
	}{
		{
			name: "append new to existing",
			existing: []state.ConversationMsg{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "response"},
			},
			newMsgs: []state.ConversationMsg{
				{Role: "user", Content: "second"},
				{Role: "assistant", Content: "reply"},
			},
			max:         50,
			wantCount:   4,
			wantLastMsg: "reply",
		},
		{
			name:      "truncates when over max",
			existing:  makeMessages(45),
			newMsgs:   makeMessages(10),
			max:       50,
			wantCount: 50,
		},
		{
			name:      "empty existing",
			existing:  nil,
			newMsgs:   makeMessages(3),
			max:       50,
			wantCount: 3,
		},
		{
			name:      "empty new",
			existing:  makeMessages(5),
			newMsgs:   nil,
			max:       50,
			wantCount: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MergeConversationHistory(tt.existing, tt.newMsgs, tt.max)
			if len(result) != tt.wantCount {
				t.Errorf("count = %d, want %d", len(result), tt.wantCount)
			}
			if tt.wantLastMsg != "" && result[len(result)-1].Content != tt.wantLastMsg {
				t.Errorf("last message = %q, want %q", result[len(result)-1].Content, tt.wantLastMsg)
			}
		})
	}
}

// ============================================================
// SummarizeCompletedWork
// ============================================================

func TestSummarizeCompletedWork(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		tasks       []state.Task
		mustContain []string
		wantEmpty   bool
	}{
		{
			name: "lists completed tasks",
			tasks: []state.Task{
				{ID: "task-001", Title: "Init project", Status: state.TaskDone},
				{ID: "task-002", Title: "Add DB", Status: state.TaskDone},
				{ID: "task-003", Title: "Pending", Status: state.TaskPending},
			},
			mustContain: []string{"task-001", "Init project", "task-002", "Add DB"},
		},
		{
			name: "no completed tasks",
			tasks: []state.Task{
				{ID: "task-001", Title: "Pending", Status: state.TaskPending},
			},
			wantEmpty: true,
		},
		{
			name:      "empty tasks",
			tasks:     nil,
			wantEmpty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := SummarizeCompletedWork(tt.tasks)
			if tt.wantEmpty && result != "" {
				t.Errorf("expected empty, got %q", result)
			}
			for _, s := range tt.mustContain {
				if !strings.Contains(result, s) {
					t.Errorf("missing %q in:\n%s", s, result)
				}
			}
		})
	}
}

// ============================================================
// SummarizePendingWork
// ============================================================

func TestSummarizePendingWork(t *testing.T) {
	t.Parallel()
	tasks := []state.Task{
		{ID: "task-001", Title: "Done", Status: state.TaskDone},
		{ID: "task-002", Title: "Auth", Status: state.TaskPending, DependsOn: []string{"task-001"}},
		{ID: "task-003", Title: "API", Status: state.TaskPending},
	}

	result := SummarizePendingWork(tasks)

	if !strings.Contains(result, "task-002") || !strings.Contains(result, "Auth") {
		t.Errorf("missing pending task in:\n%s", result)
	}
	if !strings.Contains(result, "task-003") {
		t.Errorf("missing task-003 in:\n%s", result)
	}
	if strings.Contains(result, "task-001") {
		t.Errorf("should not contain done task in pending summary:\n%s", result)
	}
}

// ============================================================
// SummarizeFailedWork
// ============================================================

func TestSummarizeFailedWork(t *testing.T) {
	t.Parallel()
	tasks := []state.Task{
		{ID: "task-001", Title: "Done", Status: state.TaskDone},
		{ID: "task-002", Title: "Payment", Status: state.TaskFailed, Retries: 3},
	}

	result := SummarizeFailedWork(tasks)

	if !strings.Contains(result, "task-002") || !strings.Contains(result, "Payment") {
		t.Errorf("missing failed task in:\n%s", result)
	}
	if !strings.Contains(result, "3") {
		t.Errorf("should mention retry count in:\n%s", result)
	}
}

// ============================================================
// Integration: Full Replan Cycle
// ============================================================

func TestFullReplanCycle(t *testing.T) {
	t.Parallel()

	// Start with a state that has completed and pending tasks
	s := &state.State{
		ProjectName: "test-api",
		PlanVersion: 1,
		PlanHistory: []state.PlanRevision{
			{Version: 1, Summary: "Initial plan"},
		},
		ConversationHistory: []state.ConversationMsg{
			{Role: "user", Content: "Build a REST API"},
			{Role: "assistant", Content: "Sure, here's the plan"},
		},
		Tasks: []state.Task{
			{ID: "task-001", Title: "Init project", Status: state.TaskDone,
				PlanVersionCreated: 1, PlanVersionModified: 1},
			{ID: "task-002", Title: "Add JWT auth", Status: state.TaskDone,
				PlanVersionCreated: 1, PlanVersionModified: 1},
			{ID: "task-003", Title: "Add GraphQL", Status: state.TaskPending,
				PlanVersionCreated: 1, PlanVersionModified: 1,
				DependsOn: []string{"task-001"}},
			{ID: "task-004", Title: "Add tests", Status: state.TaskPending,
				PlanVersionCreated: 1, PlanVersionModified: 1,
				DependsOn: []string{"task-003"}},
			{ID: "task-005", Title: "Deploy", Status: state.TaskPending,
				PlanVersionCreated: 1, PlanVersionModified: 1},
		},
	}

	// Step 1: Build replan context
	ctx := BuildReplanContext(s)
	if ctx.CompletedCount != 2 {
		t.Fatalf("CompletedCount = %d", ctx.CompletedCount)
	}
	if ctx.PendingCount != 3 {
		t.Fatalf("PendingCount = %d", ctx.PendingCount)
	}

	// Step 2: Simulate claude returning a plan update
	update := &claude.PlanUpdateJSON{
		Summary: "Replaced GraphQL with REST, added caching",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "keep"},
			{ID: "task-002", Action: "keep"},
			{ID: "task-003", Action: "remove", Reason: "Switching to REST"},
			{ID: "task-004", Action: "modify", Title: "Add REST endpoint tests",
				Description: "Test all REST endpoints",
				AcceptanceCriteria: []string{"all endpoints tested"},
				Complexity: "medium"},
			{ID: "task-005", Action: "keep"},
			{Action: "add", Title: "Add REST endpoints",
				Description: "CRUD endpoints for all resources",
				AcceptanceCriteria: []string{"CRUD works"},
				Complexity: "medium",
				DependsOn: []string{"task-002"}},
			{Action: "add", Title: "Add Redis caching",
				Description: "Cache frequent queries",
				AcceptanceCriteria: []string{"cache reduces DB load"},
				Complexity: "medium"},
		},
	}

	// Step 3: Validate
	warnings, err := ValidatePlanUpdate(s, update)
	if err != nil {
		t.Fatalf("ValidatePlanUpdate error: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("Warnings: %v", warnings)
	}

	// Step 4: Apply the update
	err = ApplyPlanUpdate(s, update)
	if err != nil {
		t.Fatalf("ApplyPlanUpdate error: %v", err)
	}

	// Step 5: Bump version
	newVersion := s.BumpPlanVersion("Replaced GraphQL with REST, added caching")

	// Step 6: Verify final state
	if newVersion != 2 {
		t.Errorf("new version = %d, want 2", newVersion)
	}

	// task-001, task-002 still done
	if s.FindTask("task-001").Status != state.TaskDone {
		t.Error("task-001 should still be done")
	}
	if s.FindTask("task-002").Status != state.TaskDone {
		t.Error("task-002 should still be done")
	}

	// task-003 cancelled
	task3 := s.FindTask("task-003")
	if task3.Status != state.TaskCancelled {
		t.Errorf("task-003 status = %q, want cancelled", task3.Status)
	}
	if task3.CancelledReason != "Switching to REST" {
		t.Errorf("task-003 reason = %q", task3.CancelledReason)
	}

	// task-004 modified
	task4 := s.FindTask("task-004")
	if task4.Title != "Add REST endpoint tests" {
		t.Errorf("task-004 title = %q", task4.Title)
	}

	// task-005 unchanged
	if s.FindTask("task-005").Title != "Deploy" {
		t.Error("task-005 should be unchanged")
	}

	// new tasks exist
	task6 := s.FindTask("task-006")
	if task6 == nil {
		t.Fatal("task-006 should exist")
	}
	if task6.Title != "Add REST endpoints" {
		t.Errorf("task-006 title = %q", task6.Title)
	}
	if len(task6.DependsOn) == 0 || task6.DependsOn[0] != "task-002" {
		t.Errorf("task-006 depends on = %v", task6.DependsOn)
	}

	task7 := s.FindTask("task-007")
	if task7 == nil {
		t.Fatal("task-007 should exist")
	}
	if task7.Title != "Add Redis caching" {
		t.Errorf("task-007 title = %q", task7.Title)
	}

	// Total active tasks: 2 done + 1 cancelled + 4 active = 7
	if len(s.Tasks) != 7 {
		t.Errorf("total tasks = %d, want 7", len(s.Tasks))
	}

	// Executable tasks should be: task-005 (no blocking deps), task-006 (depends on done task-002),
	// task-007 (no deps). task-004 depends on cancelled task-003 — should be skipped.
	executable := s.ExecutableTasks()
	execIDs := make(map[string]bool)
	for _, t2 := range executable {
		execIDs[t2.ID] = true
	}
	if execIDs["task-004"] {
		t.Error("task-004 should be blocked (depends on cancelled task-003)")
	}
	if !execIDs["task-005"] {
		t.Error("task-005 should be executable")
	}
	if !execIDs["task-006"] {
		t.Error("task-006 should be executable (depends on done task-002)")
	}
	if !execIDs["task-007"] {
		t.Error("task-007 should be executable")
	}
}

// ============================================================
// Helpers
// ============================================================

func makeMessages(n int) []state.ConversationMsg {
	msgs := make([]state.ConversationMsg, n)
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = state.ConversationMsg{Role: role, Content: fmt.Sprintf("message %d", i)}
	}
	return msgs
}

func copyState(s *state.State) *state.State {
	data, _ := json.Marshal(s)
	var cp state.State
	json.Unmarshal(data, &cp)
	return &cp
}
