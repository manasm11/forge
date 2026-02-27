package tui

import (
	"testing"

	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/state"
)

func TestApplyInitialPlan(t *testing.T) {
	t.Parallel()

	plan := &claude.PlanJSON{
		ProjectName: "test-api",
		Description: "A test API",
		TechStack:   []string{"Go", "SQLite"},
		Tasks: []claude.PlanTaskJSON{
			{
				Title:              "Init project",
				Description:        "Set up Go module",
				AcceptanceCriteria: []string{"go.mod exists"},
				Complexity:         "small",
			},
			{
				Title:              "Add user model",
				Description:        "Create user struct and DB table",
				AcceptanceCriteria: []string{"user table exists", "CRUD works"},
				DependsOn:          []int{0},
				Complexity:         "medium",
			},
		},
	}

	s := &state.State{}
	err := ApplyInitialPlan(s, plan)

	if err != nil {
		t.Fatalf("ApplyInitialPlan() error: %v", err)
	}
	if s.ProjectName != "test-api" {
		t.Errorf("ProjectName = %q", s.ProjectName)
	}
	if s.PlanVersion != 1 {
		t.Errorf("PlanVersion = %d, want 1", s.PlanVersion)
	}
	if len(s.Tasks) != 2 {
		t.Fatalf("Tasks count = %d, want 2", len(s.Tasks))
	}
	if s.Tasks[0].ID != "task-001" {
		t.Errorf("Tasks[0].ID = %q", s.Tasks[0].ID)
	}
	if s.Tasks[0].Status != state.TaskPending {
		t.Errorf("Tasks[0].Status = %q, want pending", s.Tasks[0].Status)
	}
	if s.Tasks[1].ID != "task-002" {
		t.Errorf("Tasks[1].ID = %q", s.Tasks[1].ID)
	}
	if len(s.Tasks[1].DependsOn) != 1 || s.Tasks[1].DependsOn[0] != "task-001" {
		t.Errorf("Tasks[1].DependsOn = %v, want [task-001]", s.Tasks[1].DependsOn)
	}
	if len(s.PlanHistory) != 1 {
		t.Errorf("PlanHistory length = %d, want 1", len(s.PlanHistory))
	}
}

func TestApplyInitialPlan_MissingProjectName(t *testing.T) {
	t.Parallel()
	plan := &claude.PlanJSON{
		Tasks: []claude.PlanTaskJSON{
			{Title: "t", Description: "d", AcceptanceCriteria: []string{"a"}, Complexity: "small"},
		},
	}
	s := &state.State{}
	err := ApplyInitialPlan(s, plan)
	if err == nil {
		t.Fatal("expected error for missing project_name")
	}
}

func TestApplyInitialPlan_NoTasks(t *testing.T) {
	t.Parallel()
	plan := &claude.PlanJSON{
		ProjectName: "test",
		Tasks:       []claude.PlanTaskJSON{},
	}
	s := &state.State{}
	err := ApplyInitialPlan(s, plan)
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestApplyInitialPlan_OutOfRangeDependency(t *testing.T) {
	t.Parallel()
	plan := &claude.PlanJSON{
		ProjectName: "test",
		Tasks: []claude.PlanTaskJSON{
			{Title: "Task 1", Description: "d", AcceptanceCriteria: []string{"a"}, Complexity: "small", DependsOn: []int{5}},
		},
	}
	s := &state.State{}
	err := ApplyInitialPlan(s, plan)
	if err != nil {
		t.Fatalf("should not error on out-of-range dep (just skip it): %v", err)
	}
	if len(s.Tasks[0].DependsOn) != 0 {
		t.Errorf("out-of-range dep should be skipped, got DependsOn = %v", s.Tasks[0].DependsOn)
	}
}

func TestApplyPlanUpdate_ComplexScenario(t *testing.T) {
	t.Parallel()
	s := &state.State{
		PlanVersion: 2,
		Tasks: []state.Task{
			{ID: "task-001", Title: "Init", Status: state.TaskDone},
			{ID: "task-002", Title: "Auth", Status: state.TaskDone},
			{ID: "task-003", Title: "GraphQL", Status: state.TaskPending},
			{ID: "task-004", Title: "Tests", Status: state.TaskPending, DependsOn: []string{"task-003"}},
			{ID: "task-005", Title: "Deploy", Status: state.TaskPending},
		},
	}

	update := &claude.PlanUpdateJSON{
		Summary: "Replaced GraphQL with REST, added caching",
		Tasks: []claude.PlanUpdateTaskJSON{
			{ID: "task-001", Action: "keep"},
			{ID: "task-002", Action: "keep"},
			{ID: "task-003", Action: "remove", Reason: "Switching to REST"},
			{ID: "task-004", Action: "modify", Title: "REST endpoint tests", Description: "Updated for REST",
				AcceptanceCriteria: []string{"all REST endpoints tested"}},
			{ID: "task-005", Action: "keep"},
			{Action: "add", Title: "Add REST endpoints", Description: "Replace GraphQL",
				AcceptanceCriteria: []string{"CRUD works"}, Complexity: "medium",
				DependsOn: []string{"task-002"}},
			{Action: "add", Title: "Add Redis caching", Description: "Cache layer",
				AcceptanceCriteria: []string{"cache works"}, Complexity: "medium"},
		},
	}

	err := ApplyPlanUpdate(s, update)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify outcomes
	if len(s.Tasks) != 7 { // 5 original + 2 new
		t.Fatalf("Tasks count = %d, want 7", len(s.Tasks))
	}

	task3 := s.FindTask("task-003")
	if task3.Status != state.TaskCancelled {
		t.Errorf("task-003 status = %q, want cancelled", task3.Status)
	}
	if task3.CancelledReason != "Switching to REST" {
		t.Errorf("task-003 CancelledReason = %q", task3.CancelledReason)
	}

	task4 := s.FindTask("task-004")
	if task4.Title != "REST endpoint tests" {
		t.Errorf("task-004 title = %q", task4.Title)
	}

	task6 := s.FindTask("task-006")
	if task6 == nil {
		t.Fatal("task-006 should exist")
	}
	if task6.Title != "Add REST endpoints" {
		t.Errorf("task-006 title = %q", task6.Title)
	}
	if len(task6.DependsOn) != 1 || task6.DependsOn[0] != "task-002" {
		t.Errorf("task-006 DependsOn = %v", task6.DependsOn)
	}

	task7 := s.FindTask("task-007")
	if task7 == nil {
		t.Fatal("task-007 should exist")
	}
	if task7.Title != "Add Redis caching" {
		t.Errorf("task-007 title = %q", task7.Title)
	}
}
