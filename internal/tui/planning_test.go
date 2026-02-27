package tui

import (
	"strings"
	"testing"

	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/state"
)

func TestApplyPlanUpdate_Keep(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Setup project", Status: state.TaskDone},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "No changes",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "keep"},
		},
	}

	if err := ApplyPlanUpdate(s, update); err != nil {
		t.Fatalf("ApplyPlanUpdate() error: %v", err)
	}

	task := s.FindTask("task-001")
	if task.Title != "Setup project" {
		t.Errorf("keep action should not change title, got %q", task.Title)
	}
	if task.Status != state.TaskDone {
		t.Errorf("keep action should not change status, got %q", task.Status)
	}
}

func TestApplyPlanUpdate_Modify(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{
				ID:                 "task-001",
				Title:              "Add auth",
				Description:        "Basic auth",
				AcceptanceCriteria: []string{"login works"},
				Complexity:         "small",
				Status:             state.TaskPending,
				PlanVersionModified: 1,
			},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Updated auth task",
		Tasks: []claude.PlanUpdateTaskJSON{
			{
				ID:                 "task-001",
				Action:             "modify",
				Title:              "Add JWT authentication",
				Description:        "JWT-based auth with refresh tokens",
				AcceptanceCriteria: []string{"login works", "refresh token works"},
				Complexity:         "medium",
			},
		},
	}

	if err := ApplyPlanUpdate(s, update); err != nil {
		t.Fatalf("ApplyPlanUpdate() error: %v", err)
	}

	task := s.FindTask("task-001")
	if task.Title != "Add JWT authentication" {
		t.Errorf("title = %q, want %q", task.Title, "Add JWT authentication")
	}
	if task.Description != "JWT-based auth with refresh tokens" {
		t.Errorf("description mismatch")
	}
	if len(task.AcceptanceCriteria) != 2 {
		t.Errorf("criteria count = %d, want 2", len(task.AcceptanceCriteria))
	}
	if task.Complexity != "medium" {
		t.Errorf("complexity = %q, want %q", task.Complexity, "medium")
	}
	if task.PlanVersionModified != 2 {
		t.Errorf("PlanVersionModified = %d, want 2", task.PlanVersionModified)
	}
}

func TestApplyPlanUpdate_ModifyCompletedTaskFails(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Done task", Status: state.TaskDone},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Try to modify done task",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "modify", Title: "Changed"},
		},
	}

	err := ApplyPlanUpdate(s, update)
	if err == nil {
		t.Fatal("expected error when modifying completed task")
	}
	if !strings.Contains(err.Error(), "completed") {
		t.Errorf("error should mention 'completed', got %q", err.Error())
	}
}

func TestApplyPlanUpdate_Add(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 2,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Existing", Status: state.TaskDone},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Added new task",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "keep"},
			{
				Action:             "add",
				Title:              "New feature",
				Description:        "A new feature",
				AcceptanceCriteria: []string{"it works"},
				DependsOn:          []string{"task-001"},
				Complexity:         "large",
			},
		},
	}

	if err := ApplyPlanUpdate(s, update); err != nil {
		t.Fatalf("ApplyPlanUpdate() error: %v", err)
	}

	if len(s.Tasks) != 2 {
		t.Fatalf("tasks length = %d, want 2", len(s.Tasks))
	}

	newTask := s.Tasks[1]
	if newTask.ID != "task-002" {
		t.Errorf("new task ID = %q, want %q", newTask.ID, "task-002")
	}
	if newTask.Title != "New feature" {
		t.Errorf("new task title = %q, want %q", newTask.Title, "New feature")
	}
	if newTask.Status != state.TaskPending {
		t.Errorf("new task status = %q, want %q", newTask.Status, state.TaskPending)
	}
	if len(newTask.DependsOn) != 1 || newTask.DependsOn[0] != "task-001" {
		t.Errorf("new task DependsOn = %v, want [task-001]", newTask.DependsOn)
	}
	if newTask.PlanVersionCreated != 2 {
		t.Errorf("PlanVersionCreated = %d, want 2", newTask.PlanVersionCreated)
	}
}

func TestApplyPlanUpdate_Remove(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "To remove", Status: state.TaskPending},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Removed task",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "remove", Reason: "no longer needed"},
		},
	}

	if err := ApplyPlanUpdate(s, update); err != nil {
		t.Fatalf("ApplyPlanUpdate() error: %v", err)
	}

	task := s.FindTask("task-001")
	if task.Status != state.TaskCancelled {
		t.Errorf("status = %q, want %q", task.Status, state.TaskCancelled)
	}
	if task.CancelledReason != "no longer needed" {
		t.Errorf("CancelledReason = %q, want %q", task.CancelledReason, "no longer needed")
	}
}

func TestApplyPlanUpdate_RemoveCompletedTaskFails(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Done", Status: state.TaskDone},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Try to remove done task",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "remove", Reason: "not needed"},
		},
	}

	err := ApplyPlanUpdate(s, update)
	if err == nil {
		t.Fatal("expected error when removing completed task")
	}
	if !strings.Contains(err.Error(), "completed") {
		t.Errorf("error should mention 'completed', got %q", err.Error())
	}
}

func TestApplyPlanUpdate_MixedActions(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 2,
		Tasks: []state.Task{
			{ID: "task-001", Title: "First", Status: state.TaskDone},
			{ID: "task-002", Title: "Second", Status: state.TaskPending, Complexity: "small"},
			{ID: "task-003", Title: "Third", Status: state.TaskPending},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Mixed update",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "keep"},
			{ID: "task-002", Action: "modify", Title: "Updated Second", Complexity: "large"},
			{ID: "task-003", Action: "remove", Reason: "merged into task-002"},
			{Action: "add", Title: "New Task", Description: "Something new", AcceptanceCriteria: []string{"works"}, Complexity: "medium"},
		},
	}

	if err := ApplyPlanUpdate(s, update); err != nil {
		t.Fatalf("ApplyPlanUpdate() error: %v", err)
	}

	// task-001: kept as done
	t1 := s.FindTask("task-001")
	if t1.Status != state.TaskDone {
		t.Errorf("task-001 status = %q, want done", t1.Status)
	}

	// task-002: modified
	t2 := s.FindTask("task-002")
	if t2.Title != "Updated Second" {
		t.Errorf("task-002 title = %q, want 'Updated Second'", t2.Title)
	}
	if t2.Complexity != "large" {
		t.Errorf("task-002 complexity = %q, want 'large'", t2.Complexity)
	}

	// task-003: cancelled
	t3 := s.FindTask("task-003")
	if t3.Status != state.TaskCancelled {
		t.Errorf("task-003 status = %q, want cancelled", t3.Status)
	}

	// task-004: added
	t4 := s.FindTask("task-004")
	if t4 == nil {
		t.Fatal("task-004 should have been created")
	}
	if t4.Title != "New Task" {
		t.Errorf("task-004 title = %q, want 'New Task'", t4.Title)
	}
}

func TestApplyPlanUpdate_UnknownAction(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Test", Status: state.TaskPending},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Bad action",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "invalid"},
		},
	}

	err := ApplyPlanUpdate(s, update)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("error should mention 'unknown action', got %q", err.Error())
	}
}

func TestApplyPlanUpdate_ModifyNotFound(t *testing.T) {
	t.Parallel()
	s := &state.State{PlanVersion: 1}

	update := &claude.PlanUpdateJSON{
		Summary: "Missing task",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-999", Action: "modify", Title: "Ghost"},
		},
	}

	err := ApplyPlanUpdate(s, update)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got %q", err.Error())
	}
}

func TestApplyPlanUpdate_RemoveDefaultReason(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 1,
		Tasks: []state.Task{
			{ID: "task-001", Title: "To remove", Status: state.TaskPending},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Remove without reason",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "remove"},
		},
	}

	if err := ApplyPlanUpdate(s, update); err != nil {
		t.Fatalf("ApplyPlanUpdate() error: %v", err)
	}

	task := s.FindTask("task-001")
	if task.CancelledReason != "Removed during replanning" {
		t.Errorf("CancelledReason = %q, want 'Removed during replanning'", task.CancelledReason)
	}
}

func TestFormatLOC(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{100, "100"},
		{1000, "1,000"},
		{3200, "3,200"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		got := formatLOC(tt.input)
		if got != tt.want {
			t.Errorf("formatLOC(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
