package tui

import (
	"strings"
	"testing"

	"github.com/manasm11/forge/internal/state"
)

// ============================================================
// BuildTaskDisplayList
// ============================================================

func TestBuildTaskDisplayList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		tasks     []state.Task
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "empty tasks",
			tasks:     nil,
			wantCount: 0,
			wantIDs:   nil,
		},
		{
			name: "cancelled tasks hidden",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskCancelled},
				{ID: "task-003", Status: state.TaskPending},
			},
			wantCount: 2,
			wantIDs:   []string{"task-001", "task-003"},
		},
		{
			name: "done tasks come first",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskDone},
				{ID: "task-003", Status: state.TaskPending},
			},
			wantCount: 3,
			wantIDs:   []string{"task-002", "task-001", "task-003"},
		},
		{
			name: "done tasks are not editable, pending are",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone},
				{ID: "task-002", Status: state.TaskPending},
				{ID: "task-003", Status: state.TaskInProgress},
			},
			wantCount: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			items := BuildTaskDisplayList(tt.tasks)
			if len(items) != tt.wantCount {
				t.Fatalf("count = %d, want %d", len(items), tt.wantCount)
			}
			if tt.wantIDs != nil {
				for i, wantID := range tt.wantIDs {
					if items[i].ID != wantID {
						t.Errorf("items[%d].ID = %q, want %q", i, items[i].ID, wantID)
					}
				}
			}
			// Check editability
			for _, item := range items {
				switch item.Status {
				case state.TaskPending:
					if !item.Editable {
						t.Errorf("pending task %s should be editable", item.ID)
					}
				case state.TaskDone, state.TaskInProgress:
					if item.Editable {
						t.Errorf("non-pending task %s should not be editable", item.ID)
					}
				}
			}
		})
	}
}

// ============================================================
// ReorderTask
// ============================================================

func TestReorderTask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		tasks     []state.Task
		taskID    string
		direction int // -1 = up, +1 = down
		wantOrder []string
		wantErr   bool
	}{
		{
			name: "move pending task down",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending},
				{ID: "task-003", Status: state.TaskPending},
			},
			taskID:    "task-001",
			direction: 1,
			wantOrder: []string{"task-002", "task-001", "task-003"},
			wantErr:   false,
		},
		{
			name: "move pending task up",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending},
				{ID: "task-003", Status: state.TaskPending},
			},
			taskID:    "task-003",
			direction: -1,
			wantOrder: []string{"task-001", "task-003", "task-002"},
			wantErr:   false,
		},
		{
			name: "cannot move first pending task up",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending},
			},
			taskID:    "task-001",
			direction: -1,
			wantErr:   true,
		},
		{
			name: "cannot move last pending task down",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending},
			},
			taskID:    "task-002",
			direction: 1,
			wantErr:   true,
		},
		{
			name: "cannot reorder done task",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone},
				{ID: "task-002", Status: state.TaskPending},
			},
			taskID:    "task-001",
			direction: 1,
			wantErr:   true,
		},
		{
			name: "reorder skips over done tasks",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone},
				{ID: "task-002", Status: state.TaskPending},
				{ID: "task-003", Status: state.TaskPending},
			},
			taskID:    "task-003",
			direction: -1,
			wantOrder: []string{"task-001", "task-003", "task-002"},
			wantErr:   false,
		},
		{
			name:      "nonexistent task",
			tasks:     []state.Task{{ID: "task-001", Status: state.TaskPending}},
			taskID:    "task-999",
			direction: 1,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := ReorderTask(tt.tasks, tt.taskID, tt.direction)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.wantOrder != nil {
				ids := make([]string, len(result))
				for i, task := range result {
					ids[i] = task.ID
				}
				for i, wantID := range tt.wantOrder {
					if ids[i] != wantID {
						t.Errorf("position %d = %q, want %q (full: %v)", i, ids[i], wantID, ids)
						break
					}
				}
			}
		})
	}
}

// ============================================================
// DeleteTask
// ============================================================

func TestDeleteTask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		tasks       []state.Task
		deleteID    string
		wantCount   int
		wantErr     bool
		checkDepsOf string   // verify this task's DependsOn was cleaned up
		wantDeps    []string // expected DependsOn after cleanup
	}{
		{
			name: "delete pending task",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending},
			},
			deleteID:  "task-001",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "cannot delete done task",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone},
			},
			deleteID: "task-001",
			wantErr:  true,
		},
		{
			name: "cannot delete in-progress task",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskInProgress},
			},
			deleteID: "task-001",
			wantErr:  true,
		},
		{
			name: "nonexistent task",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
			},
			deleteID: "task-999",
			wantErr:  true,
		},
		{
			name: "deleting task cleans up dependencies",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending, DependsOn: []string{"task-001", "task-003"}},
				{ID: "task-003", Status: state.TaskPending},
			},
			deleteID:    "task-001",
			wantCount:   2,
			wantErr:     false,
			checkDepsOf: "task-002",
			wantDeps:    []string{"task-003"},
		},
		{
			name: "deleting task removes it as sole dependency",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending, DependsOn: []string{"task-001"}},
			},
			deleteID:    "task-001",
			wantCount:   1,
			wantErr:     false,
			checkDepsOf: "task-002",
			wantDeps:    nil, // empty after removal
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := DeleteTask(tt.tasks, tt.deleteID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(result) != tt.wantCount {
					t.Errorf("count = %d, want %d", len(result), tt.wantCount)
				}
				if tt.checkDepsOf != "" {
					for _, task := range result {
						if task.ID == tt.checkDepsOf {
							if len(tt.wantDeps) == 0 && len(task.DependsOn) == 0 {
								break // both empty, ok
							}
							if len(task.DependsOn) != len(tt.wantDeps) {
								t.Errorf("%s.DependsOn = %v, want %v", tt.checkDepsOf, task.DependsOn, tt.wantDeps)
							}
							break
						}
					}
				}
			}
		})
	}
}

// ============================================================
// ValidateNewTask
// ============================================================

func TestValidateNewTask(t *testing.T) {
	t.Parallel()
	existing := []state.Task{
		{ID: "task-001", Status: state.TaskDone},
		{ID: "task-002", Status: state.TaskPending},
	}
	tests := []struct {
		name       string
		title      string
		desc       string
		complexity string
		criteria   []string
		dependsOn  []string
		wantErr    bool
	}{
		{
			name:       "valid task",
			title:      "New feature",
			desc:       "Do something",
			complexity: "medium",
			criteria:   []string{"it works"},
			wantErr:    false,
		},
		{
			name:       "empty title",
			title:      "",
			complexity: "small",
			wantErr:    true,
		},
		{
			name:       "whitespace only title",
			title:      "   ",
			complexity: "small",
			wantErr:    true,
		},
		{
			name:       "invalid complexity",
			title:      "Task",
			complexity: "huge",
			wantErr:    true,
		},
		{
			name:       "valid dependency",
			title:      "Task",
			complexity: "small",
			dependsOn:  []string{"task-001"},
			wantErr:    false,
		},
		{
			name:       "nonexistent dependency",
			title:      "Task",
			complexity: "small",
			dependsOn:  []string{"task-999"},
			wantErr:    true,
		},
		{
			name:       "empty complexity defaults ok",
			title:      "Task",
			complexity: "",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateNewTask(existing, tt.title, tt.desc, tt.complexity, tt.criteria, tt.dependsOn)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================
// FormatTaskDetail
// ============================================================

func TestFormatTaskDetail(t *testing.T) {
	t.Parallel()
	allTasks := []state.Task{
		{ID: "task-001", Title: "Init project", Status: state.TaskDone},
		{
			ID: "task-002", Title: "Add auth", Status: state.TaskPending,
			Description: "Implement JWT authentication",
			Complexity:  "medium",
			DependsOn:   []string{"task-001"},
			AcceptanceCriteria: []string{"Login works", "Token validates"},
		},
	}

	detail := FormatTaskDetail(allTasks[1], allTasks)

	mustContain := []string{
		"Add auth",
		"medium",
		"Implement JWT authentication",
		"Init project", // resolved dependency title
		"Login works",
		"Token validates",
	}
	for _, s := range mustContain {
		if !strings.Contains(detail, s) {
			t.Errorf("detail missing %q\ngot: %s", s, detail)
		}
	}
}

func TestFormatTaskDetail_NoDependencies(t *testing.T) {
	t.Parallel()
	task := state.Task{
		ID: "task-001", Title: "Init", Status: state.TaskPending,
		Description: "Set up project", Complexity: "small",
		AcceptanceCriteria: []string{"go.mod exists"},
	}

	detail := FormatTaskDetail(task, []state.Task{task})

	if strings.Contains(detail, "Depends on") {
		t.Error("should not show dependencies section when there are none")
	}
}

// ============================================================
// ResolveDependencyTitles
// ============================================================

func TestResolveDependencyTitles(t *testing.T) {
	t.Parallel()
	tasks := []state.Task{
		{ID: "task-001", Title: "Init"},
		{ID: "task-002", Title: "Auth"},
	}

	tests := []struct {
		name      string
		dependsOn []string
		wantCount int
		wantFirst string
	}{
		{
			name:      "resolve existing",
			dependsOn: []string{"task-001"},
			wantCount: 1,
			wantFirst: "task-001: Init",
		},
		{
			name:      "unknown ID",
			dependsOn: []string{"task-999"},
			wantCount: 1,
			wantFirst: "task-999: (unknown)",
		},
		{
			name:      "empty",
			dependsOn: nil,
			wantCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ResolveDependencyTitles(tt.dependsOn, tasks)
			if len(result) != tt.wantCount {
				t.Fatalf("count = %d, want %d", len(result), tt.wantCount)
			}
			if tt.wantCount > 0 && result[0] != tt.wantFirst {
				t.Errorf("first = %q, want %q", result[0], tt.wantFirst)
			}
		})
	}
}

// ============================================================
// ComputeTaskStats
// ============================================================

func TestComputeTaskStats(t *testing.T) {
	t.Parallel()
	tasks := []state.Task{
		{Status: state.TaskDone},
		{Status: state.TaskDone},
		{Status: state.TaskPending},
		{Status: state.TaskPending},
		{Status: state.TaskPending},
		{Status: state.TaskFailed},
		{Status: state.TaskCancelled},
	}

	stats := ComputeTaskStats(tasks)

	if stats.Total != 7 {
		t.Errorf("Total = %d", stats.Total)
	}
	if stats.Done != 2 {
		t.Errorf("Done = %d", stats.Done)
	}
	if stats.Pending != 3 {
		t.Errorf("Pending = %d", stats.Pending)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d", stats.Failed)
	}
	if stats.Cancelled != 1 {
		t.Errorf("Cancelled = %d", stats.Cancelled)
	}
}

// ============================================================
// CanConfirm
// ============================================================

func TestCanConfirm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		tasks  []state.Task
		wantOK bool
	}{
		{
			name: "valid — has pending tasks",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone},
				{ID: "task-002", Status: state.TaskPending},
			},
			wantOK: true,
		},
		{
			name: "invalid — no pending tasks",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone},
			},
			wantOK: false,
		},
		{
			name:   "invalid — empty task list",
			tasks:  nil,
			wantOK: false,
		},
		{
			name: "invalid — circular dependency",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending, DependsOn: []string{"task-002"}},
				{ID: "task-002", Status: state.TaskPending, DependsOn: []string{"task-001"}},
			},
			wantOK: false,
		},
		{
			name: "valid — all cancelled except some pending",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskCancelled},
				{ID: "task-002", Status: state.TaskPending},
			},
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := CanConfirm(tt.tasks)
			if tt.wantOK && result != "" {
				t.Errorf("expected valid, got error: %q", result)
			}
			if !tt.wantOK && result == "" {
				t.Error("expected error, got valid")
			}
		})
	}
}

// ============================================================
// DetectCircularDependencies
// ============================================================

func TestDetectCircularDependencies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		tasks     []state.Task
		wantCycle bool
	}{
		{
			name: "no cycle",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending},
				{ID: "task-002", Status: state.TaskPending, DependsOn: []string{"task-001"}},
				{ID: "task-003", Status: state.TaskPending, DependsOn: []string{"task-002"}},
			},
			wantCycle: false,
		},
		{
			name: "direct cycle",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending, DependsOn: []string{"task-002"}},
				{ID: "task-002", Status: state.TaskPending, DependsOn: []string{"task-001"}},
			},
			wantCycle: true,
		},
		{
			name: "indirect cycle",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending, DependsOn: []string{"task-003"}},
				{ID: "task-002", Status: state.TaskPending, DependsOn: []string{"task-001"}},
				{ID: "task-003", Status: state.TaskPending, DependsOn: []string{"task-002"}},
			},
			wantCycle: true,
		},
		{
			name: "self-referencing",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskPending, DependsOn: []string{"task-001"}},
			},
			wantCycle: true,
		},
		{
			name: "cycle only in pending tasks — done tasks ignored",
			tasks: []state.Task{
				{ID: "task-001", Status: state.TaskDone, DependsOn: []string{"task-002"}},
				{ID: "task-002", Status: state.TaskDone, DependsOn: []string{"task-001"}},
				{ID: "task-003", Status: state.TaskPending},
			},
			wantCycle: false,
		},
		{
			name:      "empty tasks",
			tasks:     nil,
			wantCycle: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cycle := DetectCircularDependencies(tt.tasks)
			if tt.wantCycle && len(cycle) == 0 {
				t.Error("expected cycle, got none")
			}
			if !tt.wantCycle && len(cycle) > 0 {
				t.Errorf("expected no cycle, got %v", cycle)
			}
		})
	}
}
