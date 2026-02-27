package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestForgeDir(t *testing.T) {
	got := ForgeDir("/some/root")
	want := filepath.Join("/some/root", ".forge")
	if got != want {
		t.Errorf("ForgeDir() = %q, want %q", got, want)
	}
}

func TestInit(t *testing.T) {
	t.Run("creates state with correct defaults", func(t *testing.T) {
		root := t.TempDir()

		s, err := Init(root)
		if err != nil {
			t.Fatalf("Init() error: %v", err)
		}

		if s.Phase != PhasePlanning {
			t.Errorf("Phase = %q, want %q", s.Phase, PhasePlanning)
		}
		if s.PlanVersion != 0 {
			t.Errorf("PlanVersion = %d, want 0", s.PlanVersion)
		}
		if s.CreatedAt.IsZero() {
			t.Error("CreatedAt should not be zero")
		}
		if s.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should not be zero")
		}

		// Verify file was created
		path := filepath.Join(ForgeDir(root), stateFileName)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("state file not created: %v", err)
		}
	})

	t.Run("fails if state already exists", func(t *testing.T) {
		root := t.TempDir()

		if _, err := Init(root); err != nil {
			t.Fatalf("first Init() error: %v", err)
		}

		_, err := Init(root)
		if err == nil {
			t.Fatal("second Init() should have returned an error")
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("returns nil nil when no state file", func(t *testing.T) {
		root := t.TempDir()

		s, err := Load(root)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if s != nil {
			t.Fatal("Load() should return nil when no state file exists")
		}
	})

	t.Run("reads back what Save wrote", func(t *testing.T) {
		root := t.TempDir()

		original := &State{
			ProjectName: "test-project",
			Phase:       PhaseReview,
			PlanVersion: 2,
			PlanHistory: []PlanRevision{
				{Version: 1, Summary: "Initial plan", Timestamp: time.Now()},
				{Version: 2, Summary: "Added caching", Timestamp: time.Now()},
			},
			ConversationHistory: []ConversationMsg{
				{Role: "user", Content: "Build me an API"},
				{Role: "assistant", Content: "Sure, let me help plan that."},
			},
			Tasks: []Task{
				{
					ID:                 "task-001",
					Title:              "Setup project",
					Description:        "Initialize the project",
					AcceptanceCriteria: []string{"go build passes"},
					Complexity:         "small",
					Status:             TaskDone,
					PlanVersionCreated: 1,
				},
			},
			Settings: &Settings{
				TestCommand:   "go test ./...",
				BranchPattern: "forge/{{number}}-{{slug}}",
				MaxRetries:    3,
				AutoPR:        true,
			},
			CreatedAt: time.Now().Add(-time.Hour),
		}

		if err := Save(root, original); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		loaded, err := Load(root)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		if loaded.ProjectName != original.ProjectName {
			t.Errorf("ProjectName = %q, want %q", loaded.ProjectName, original.ProjectName)
		}
		if loaded.Phase != original.Phase {
			t.Errorf("Phase = %q, want %q", loaded.Phase, original.Phase)
		}
		if loaded.PlanVersion != original.PlanVersion {
			t.Errorf("PlanVersion = %d, want %d", loaded.PlanVersion, original.PlanVersion)
		}
		if len(loaded.PlanHistory) != 2 {
			t.Fatalf("PlanHistory length = %d, want 2", len(loaded.PlanHistory))
		}
		if len(loaded.ConversationHistory) != 2 {
			t.Fatalf("ConversationHistory length = %d, want 2", len(loaded.ConversationHistory))
		}
		if loaded.ConversationHistory[0].Role != "user" {
			t.Errorf("ConversationHistory[0].Role = %q, want %q", loaded.ConversationHistory[0].Role, "user")
		}
		if loaded.Settings == nil {
			t.Fatal("Settings should not be nil")
		}
		if loaded.Settings.TestCommand != original.Settings.TestCommand {
			t.Errorf("Settings.TestCommand = %q, want %q", loaded.Settings.TestCommand, original.Settings.TestCommand)
		}
		if len(loaded.Tasks) != 1 {
			t.Fatalf("Tasks length = %d, want 1", len(loaded.Tasks))
		}
		if loaded.Tasks[0].ID != "task-001" {
			t.Errorf("Tasks[0].ID = %q, want %q", loaded.Tasks[0].ID, "task-001")
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("updates UpdatedAt", func(t *testing.T) {
		root := t.TempDir()

		before := time.Now().Add(-time.Second)
		s := &State{
			Phase:     PhasePlanning,
			CreatedAt: before,
			UpdatedAt: before,
		}

		if err := Save(root, s); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		if !s.UpdatedAt.After(before) {
			t.Error("Save() should update UpdatedAt to a later time")
		}
	})

	t.Run("creates .forge directory if needed", func(t *testing.T) {
		root := t.TempDir()

		s := &State{Phase: PhasePlanning, CreatedAt: time.Now()}
		if err := Save(root, s); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		dir := ForgeDir(root)
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf(".forge directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error(".forge should be a directory")
		}
	})
}

func TestNextTaskID(t *testing.T) {
	tests := []struct {
		name  string
		tasks []Task
		want  string
	}{
		{
			name:  "no tasks",
			tasks: nil,
			want:  "task-001",
		},
		{
			name: "one task",
			tasks: []Task{
				{ID: "task-001"},
			},
			want: "task-002",
		},
		{
			name: "three tasks",
			tasks: []Task{
				{ID: "task-001"},
				{ID: "task-002"},
				{ID: "task-003"},
			},
			want: "task-004",
		},
		{
			name: "handles gaps — uses max ID not count",
			tasks: []Task{
				{ID: "task-001"},
				{ID: "task-005"},
			},
			want: "task-006",
		},
		{
			name: "handles non-sequential IDs",
			tasks: []Task{
				{ID: "task-010"},
				{ID: "task-003"},
			},
			want: "task-011",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &State{Tasks: tt.tasks}
			got := s.NextTaskID()
			if got != tt.want {
				t.Errorf("NextTaskID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddTask(t *testing.T) {
	s := &State{PlanVersion: 2}

	task := s.AddTask(
		"Setup project",
		"Initialize Go module",
		"small",
		[]string{"go.mod exists", "go build passes"},
		nil,
	)

	if task.ID != "task-001" {
		t.Errorf("ID = %q, want %q", task.ID, "task-001")
	}
	if task.Status != TaskPending {
		t.Errorf("Status = %q, want %q", task.Status, TaskPending)
	}
	if task.PlanVersionCreated != 2 {
		t.Errorf("PlanVersionCreated = %d, want 2", task.PlanVersionCreated)
	}
	if task.PlanVersionModified != 2 {
		t.Errorf("PlanVersionModified = %d, want 2", task.PlanVersionModified)
	}
	if len(task.AcceptanceCriteria) != 2 {
		t.Errorf("AcceptanceCriteria length = %d, want 2", len(task.AcceptanceCriteria))
	}

	// Add a second task
	task2 := s.AddTask("Add auth", "JWT auth", "medium", []string{"login works"}, []string{"task-001"})
	if task2.ID != "task-002" {
		t.Errorf("second task ID = %q, want %q", task2.ID, "task-002")
	}
	if len(task2.DependsOn) != 1 || task2.DependsOn[0] != "task-001" {
		t.Errorf("DependsOn = %v, want [task-001]", task2.DependsOn)
	}

	if len(s.Tasks) != 2 {
		t.Errorf("Tasks length = %d, want 2", len(s.Tasks))
	}
}

func TestFilterMethods(t *testing.T) {
	s := &State{
		Tasks: []Task{
			{ID: "task-001", Status: TaskDone, Title: "Done task"},
			{ID: "task-002", Status: TaskPending, Title: "Pending task"},
			{ID: "task-003", Status: TaskFailed, Title: "Failed task"},
			{ID: "task-004", Status: TaskCancelled, Title: "Cancelled task"},
			{ID: "task-005", Status: TaskSkipped, Title: "Skipped task"},
			{ID: "task-006", Status: TaskInProgress, Title: "In progress task"},
			{ID: "task-007", Status: TaskPending, Title: "Another pending"},
		},
	}

	t.Run("PendingTasks", func(t *testing.T) {
		pending := s.PendingTasks()
		if len(pending) != 2 {
			t.Fatalf("PendingTasks() length = %d, want 2", len(pending))
		}
		if pending[0].ID != "task-002" || pending[1].ID != "task-007" {
			t.Errorf("unexpected pending tasks: %v", pending)
		}
	})

	t.Run("CompletedTasks", func(t *testing.T) {
		done := s.CompletedTasks()
		if len(done) != 1 {
			t.Fatalf("CompletedTasks() length = %d, want 1", len(done))
		}
		if done[0].ID != "task-001" {
			t.Errorf("unexpected completed task: %s", done[0].ID)
		}
	})

	t.Run("FailedTasks", func(t *testing.T) {
		failed := s.FailedTasks()
		if len(failed) != 1 {
			t.Fatalf("FailedTasks() length = %d, want 1", len(failed))
		}
		if failed[0].ID != "task-003" {
			t.Errorf("unexpected failed task: %s", failed[0].ID)
		}
	})

	t.Run("ActiveTasks", func(t *testing.T) {
		active := s.ActiveTasks()
		if len(active) != 5 {
			t.Fatalf("ActiveTasks() length = %d, want 5", len(active))
		}
		// Should include done, pending, failed, in-progress — but not cancelled or skipped
		for _, task := range active {
			if task.Status == TaskCancelled || task.Status == TaskSkipped {
				t.Errorf("ActiveTasks() should not include %s task %s", task.Status, task.ID)
			}
		}
	})
}

func TestFindTask(t *testing.T) {
	s := &State{
		Tasks: []Task{
			{ID: "task-001", Title: "First"},
			{ID: "task-002", Title: "Second"},
		},
	}

	t.Run("finds existing task", func(t *testing.T) {
		task := s.FindTask("task-002")
		if task == nil {
			t.Fatal("FindTask() returned nil for existing task")
		}
		if task.Title != "Second" {
			t.Errorf("Title = %q, want %q", task.Title, "Second")
		}
	})

	t.Run("returns nil for missing task", func(t *testing.T) {
		task := s.FindTask("task-999")
		if task != nil {
			t.Errorf("FindTask() should return nil for missing task, got %v", task)
		}
	})

	t.Run("returns mutable pointer", func(t *testing.T) {
		task := s.FindTask("task-001")
		task.Title = "Modified"
		if s.Tasks[0].Title != "Modified" {
			t.Error("FindTask() should return a pointer to the actual task")
		}
	})
}

func TestCancelTask(t *testing.T) {
	tests := []struct {
		name      string
		status    TaskStatus
		wantErr   bool
		errSubstr string
	}{
		{name: "cancel pending task", status: TaskPending, wantErr: false},
		{name: "cancel failed task", status: TaskFailed, wantErr: false},
		{name: "cancel skipped task", status: TaskSkipped, wantErr: false},
		{name: "cannot cancel done task", status: TaskDone, wantErr: true, errSubstr: "already done"},
		{name: "cannot cancel in-progress task", status: TaskInProgress, wantErr: true, errSubstr: "in progress"},
		{name: "cannot cancel already cancelled task", status: TaskCancelled, wantErr: true, errSubstr: "already cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &State{
				Tasks: []Task{
					{ID: "task-001", Status: tt.status},
				},
			}

			err := s.CancelTask("task-001", "no longer needed")
			if tt.wantErr {
				if err == nil {
					t.Fatal("CancelTask() should have returned an error")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Fatalf("CancelTask() unexpected error: %v", err)
				}
				task := s.FindTask("task-001")
				if task.Status != TaskCancelled {
					t.Errorf("Status = %q, want %q", task.Status, TaskCancelled)
				}
				if task.CancelledReason != "no longer needed" {
					t.Errorf("CancelledReason = %q, want %q", task.CancelledReason, "no longer needed")
				}
			}
		})
	}

	t.Run("task not found", func(t *testing.T) {
		s := &State{}
		err := s.CancelTask("task-999", "reason")
		if err == nil {
			t.Fatal("CancelTask() should error for missing task")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error %q should contain 'not found'", err.Error())
		}
	})
}

func TestBumpPlanVersion(t *testing.T) {
	s := &State{PlanVersion: 0}

	v1 := s.BumpPlanVersion("Initial plan")
	if v1 != 1 {
		t.Errorf("first bump returned %d, want 1", v1)
	}
	if s.PlanVersion != 1 {
		t.Errorf("PlanVersion = %d, want 1", s.PlanVersion)
	}
	if len(s.PlanHistory) != 1 {
		t.Fatalf("PlanHistory length = %d, want 1", len(s.PlanHistory))
	}
	if s.PlanHistory[0].Version != 1 {
		t.Errorf("PlanHistory[0].Version = %d, want 1", s.PlanHistory[0].Version)
	}
	if s.PlanHistory[0].Summary != "Initial plan" {
		t.Errorf("PlanHistory[0].Summary = %q, want %q", s.PlanHistory[0].Summary, "Initial plan")
	}
	if s.PlanHistory[0].Timestamp.IsZero() {
		t.Error("PlanHistory[0].Timestamp should not be zero")
	}

	v2 := s.BumpPlanVersion("Added caching")
	if v2 != 2 {
		t.Errorf("second bump returned %d, want 2", v2)
	}
	if len(s.PlanHistory) != 2 {
		t.Fatalf("PlanHistory length = %d, want 2", len(s.PlanHistory))
	}
}

func TestAddConversationMessage(t *testing.T) {
	t.Run("appends messages", func(t *testing.T) {
		s := &State{}

		s.AddConversationMessage("user", "Hello")
		s.AddConversationMessage("assistant", "Hi there")

		if len(s.ConversationHistory) != 2 {
			t.Fatalf("ConversationHistory length = %d, want 2", len(s.ConversationHistory))
		}
		if s.ConversationHistory[0].Role != "user" {
			t.Errorf("ConversationHistory[0].Role = %q, want %q", s.ConversationHistory[0].Role, "user")
		}
		if s.ConversationHistory[1].Content != "Hi there" {
			t.Errorf("ConversationHistory[1].Content = %q, want %q", s.ConversationHistory[1].Content, "Hi there")
		}
	})

	t.Run("auto-trims at 50 messages", func(t *testing.T) {
		s := &State{}

		// Add 51 messages to trigger auto-trim
		for i := 0; i < 51; i++ {
			s.AddConversationMessage("user", "message")
		}

		// Should have been trimmed: 1 summary + 30 recent = 31
		if len(s.ConversationHistory) != 31 {
			t.Errorf("ConversationHistory length = %d, want 31 (1 summary + 30 recent)", len(s.ConversationHistory))
		}
		if s.ConversationHistory[0].Role != "system" {
			t.Errorf("first message should be system summary, got role %q", s.ConversationHistory[0].Role)
		}
		if !strings.Contains(s.ConversationHistory[0].Content, "truncated") {
			t.Errorf("summary message should contain 'truncated', got %q", s.ConversationHistory[0].Content)
		}
	})
}

func TestTrimConversationHistory(t *testing.T) {
	t.Run("no-op when under limit", func(t *testing.T) {
		s := &State{
			ConversationHistory: []ConversationMsg{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi"},
			},
		}

		s.TrimConversationHistory(10)
		if len(s.ConversationHistory) != 2 {
			t.Errorf("length = %d, want 2", len(s.ConversationHistory))
		}
	})

	t.Run("trims and summarizes", func(t *testing.T) {
		s := &State{}
		for i := 0; i < 20; i++ {
			s.ConversationHistory = append(s.ConversationHistory, ConversationMsg{
				Role:    "user",
				Content: "message",
			})
		}

		s.TrimConversationHistory(10)

		// Should be: 1 summary + 10 recent = 11
		if len(s.ConversationHistory) != 11 {
			t.Errorf("length = %d, want 11", len(s.ConversationHistory))
		}
		if s.ConversationHistory[0].Role != "system" {
			t.Errorf("first message role = %q, want %q", s.ConversationHistory[0].Role, "system")
		}
		if !strings.Contains(s.ConversationHistory[0].Content, "10 messages removed") {
			t.Errorf("summary should mention 10 messages removed, got %q", s.ConversationHistory[0].Content)
		}
	})

	t.Run("exact limit is no-op", func(t *testing.T) {
		s := &State{
			ConversationHistory: []ConversationMsg{
				{Role: "user", Content: "one"},
				{Role: "user", Content: "two"},
				{Role: "user", Content: "three"},
			},
		}

		s.TrimConversationHistory(3)
		if len(s.ConversationHistory) != 3 {
			t.Errorf("length = %d, want 3", len(s.ConversationHistory))
		}
	})
}

func TestExecutableTasks(t *testing.T) {
	t.Run("no dependencies — all pending are executable", func(t *testing.T) {
		s := &State{
			Tasks: []Task{
				{ID: "task-001", Status: TaskPending},
				{ID: "task-002", Status: TaskPending},
			},
		}

		exec := s.ExecutableTasks()
		if len(exec) != 2 {
			t.Errorf("ExecutableTasks() length = %d, want 2", len(exec))
		}
	})

	t.Run("respects dependency order", func(t *testing.T) {
		s := &State{
			Tasks: []Task{
				{ID: "task-001", Status: TaskDone},
				{ID: "task-002", Status: TaskPending, DependsOn: []string{"task-001"}},
				{ID: "task-003", Status: TaskPending, DependsOn: []string{"task-002"}},
			},
		}

		exec := s.ExecutableTasks()
		if len(exec) != 1 {
			t.Fatalf("ExecutableTasks() length = %d, want 1", len(exec))
		}
		if exec[0].ID != "task-002" {
			t.Errorf("executable task ID = %q, want %q", exec[0].ID, "task-002")
		}
	})

	t.Run("skips tasks with failed dependencies", func(t *testing.T) {
		s := &State{
			Tasks: []Task{
				{ID: "task-001", Status: TaskFailed},
				{ID: "task-002", Status: TaskPending, DependsOn: []string{"task-001"}},
				{ID: "task-003", Status: TaskPending},
			},
		}

		exec := s.ExecutableTasks()
		if len(exec) != 1 {
			t.Fatalf("ExecutableTasks() length = %d, want 1", len(exec))
		}
		if exec[0].ID != "task-003" {
			t.Errorf("executable task ID = %q, want %q", exec[0].ID, "task-003")
		}
		// task-002 should now be skipped
		task2 := s.FindTask("task-002")
		if task2.Status != TaskSkipped {
			t.Errorf("task-002 status = %q, want %q", task2.Status, TaskSkipped)
		}
	})

	t.Run("skips tasks with cancelled dependencies", func(t *testing.T) {
		s := &State{
			Tasks: []Task{
				{ID: "task-001", Status: TaskCancelled},
				{ID: "task-002", Status: TaskPending, DependsOn: []string{"task-001"}},
			},
		}

		exec := s.ExecutableTasks()
		if len(exec) != 0 {
			t.Errorf("ExecutableTasks() length = %d, want 0", len(exec))
		}
		task2 := s.FindTask("task-002")
		if task2.Status != TaskSkipped {
			t.Errorf("task-002 status = %q, want %q", task2.Status, TaskSkipped)
		}
	})

	t.Run("waits for in-progress dependencies", func(t *testing.T) {
		s := &State{
			Tasks: []Task{
				{ID: "task-001", Status: TaskInProgress},
				{ID: "task-002", Status: TaskPending, DependsOn: []string{"task-001"}},
			},
		}

		exec := s.ExecutableTasks()
		if len(exec) != 0 {
			t.Errorf("ExecutableTasks() length = %d, want 0", len(exec))
		}
	})
}

func TestGenerateReplanContext(t *testing.T) {
	s := &State{
		ProjectName: "my-api",
		PlanVersion: 3,
		Tasks: []Task{
			{ID: "task-001", Title: "Initialize Go project", Status: TaskDone},
			{ID: "task-002", Title: "Add user authentication with JWT", Status: TaskDone},
			{ID: "task-003", Title: "Add GraphQL endpoint", Status: TaskCancelled, CancelledReason: "Replaced by REST in plan v2"},
			{ID: "task-004", Title: "Add payment integration", Status: TaskFailed, Retries: 3},
			{ID: "task-005", Title: "Add order management endpoints", Status: TaskPending},
			{ID: "task-006", Title: "Add WebSocket notifications", Status: TaskPending},
		},
	}

	ctx := s.GenerateReplanContext()

	// Check all sections are present
	checks := []struct {
		name    string
		substr  string
	}{
		{"plan version", "Plan version: 3"},
		{"project name", "Project: my-api"},
		{"completed header", "COMPLETED TASKS"},
		{"completed task-001", "task-001: Initialize Go project"},
		{"completed task-002", "task-002: Add user authentication with JWT"},
		{"pending header", "PENDING TASKS"},
		{"pending task-005", "task-005: Add order management endpoints"},
		{"pending task-006", "task-006: Add WebSocket notifications"},
		{"failed header", "FAILED TASKS"},
		{"failed task-004", "task-004: Add payment integration (failed after 3 retries)"},
		{"cancelled header", "CANCELLED TASKS"},
		{"cancelled task-003", "task-003: Add GraphQL endpoint (Replaced by REST in plan v2)"},
		{"instruction keep", "Keep all completed tasks"},
		{"instruction plan_update", "<plan_update>"},
	}

	for _, c := range checks {
		if !strings.Contains(ctx, c.substr) {
			t.Errorf("GenerateReplanContext() missing %s: should contain %q", c.name, c.substr)
		}
	}
}

func TestGenerateReplanContext_Empty(t *testing.T) {
	s := &State{PlanVersion: 1}
	ctx := s.GenerateReplanContext()

	if !strings.Contains(ctx, "Plan version: 1") {
		t.Error("should contain plan version even with no tasks")
	}
	// Should not contain task section headers when no tasks of that type exist
	if strings.Contains(ctx, "COMPLETED TASKS") {
		t.Error("should not contain COMPLETED TASKS header when there are none")
	}
}

func TestRoundTrip(t *testing.T) {
	root := t.TempDir()

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	original := &State{
		ProjectName: "round-trip-test",
		Phase:       PhaseExecution,
		PlanVersion: 3,
		PlanHistory: []PlanRevision{
			{Version: 1, Summary: "Initial plan", Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			{Version: 2, Summary: "Added caching", Timestamp: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)},
			{Version: 3, Summary: "Removed GraphQL", Timestamp: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)},
		},
		ConversationHistory: []ConversationMsg{
			{Role: "user", Content: "Build a REST API"},
			{Role: "assistant", Content: "Sure, let me plan that."},
			{Role: "system", Content: "[context]"},
		},
		Settings: &Settings{
			TestCommand:   "make test",
			BuildCommand:  "make build",
			BranchPattern: "forge/{{number}}-{{slug}}",
			MaxRetries:    5,
			AutoPR:        true,
			EnvVars:       map[string]string{"GO_ENV": "test"},
			ExtraContext:  "Some extra context",
		},
		Tasks: []Task{
			{
				ID:                  "task-001",
				Title:               "First task",
				Description:         "Do the first thing",
				AcceptanceCriteria:  []string{"criterion 1", "criterion 2"},
				Complexity:          "small",
				Status:              TaskDone,
				PlanVersionCreated:  1,
				PlanVersionModified: 1,
				Branch:              "forge/1-first-task",
				GitSHA:              "abc123",
				CompletedAt:         &completedAt,
			},
			{
				ID:                  "task-002",
				Title:               "Second task",
				Description:         "Do the second thing",
				AcceptanceCriteria:  []string{"criterion A"},
				DependsOn:           []string{"task-001"},
				Complexity:          "large",
				Status:              TaskInProgress,
				PlanVersionCreated:  1,
				PlanVersionModified: 2,
				Retries:             2,
			},
			{
				ID:                  "task-003",
				Title:               "Cancelled task",
				Description:         "Was removed",
				AcceptanceCriteria:  []string{},
				Complexity:          "medium",
				Status:              TaskCancelled,
				PlanVersionCreated:  1,
				PlanVersionModified: 2,
				CancelledReason:     "No longer needed",
			},
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := Save(root, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify all fields survived the round trip
	if loaded.ProjectName != original.ProjectName {
		t.Errorf("ProjectName mismatch: got %q, want %q", loaded.ProjectName, original.ProjectName)
	}
	if loaded.Phase != original.Phase {
		t.Errorf("Phase mismatch: got %q, want %q", loaded.Phase, original.Phase)
	}
	if loaded.PlanVersion != original.PlanVersion {
		t.Errorf("PlanVersion mismatch: got %d, want %d", loaded.PlanVersion, original.PlanVersion)
	}
	if len(loaded.PlanHistory) != 3 {
		t.Fatalf("PlanHistory length = %d, want 3", len(loaded.PlanHistory))
	}
	if loaded.PlanHistory[2].Summary != "Removed GraphQL" {
		t.Errorf("PlanHistory[2].Summary = %q, want %q", loaded.PlanHistory[2].Summary, "Removed GraphQL")
	}
	if len(loaded.ConversationHistory) != 3 {
		t.Fatalf("ConversationHistory length = %d, want 3", len(loaded.ConversationHistory))
	}
	if loaded.ConversationHistory[0].Role != "user" {
		t.Errorf("ConversationHistory[0].Role = %q, want %q", loaded.ConversationHistory[0].Role, "user")
	}
	if loaded.Settings.ExtraContext != original.Settings.ExtraContext {
		t.Errorf("Settings.ExtraContext mismatch")
	}
	if loaded.Settings.EnvVars["GO_ENV"] != "test" {
		t.Errorf("Settings.EnvVars[GO_ENV] = %q, want %q", loaded.Settings.EnvVars["GO_ENV"], "test")
	}
	if len(loaded.Tasks) != 3 {
		t.Fatalf("Tasks length = %d, want 3", len(loaded.Tasks))
	}
	if loaded.Tasks[0].ID != "task-001" {
		t.Errorf("Tasks[0].ID = %q, want %q", loaded.Tasks[0].ID, "task-001")
	}
	if loaded.Tasks[0].GitSHA != "abc123" {
		t.Errorf("Tasks[0].GitSHA = %q, want %q", loaded.Tasks[0].GitSHA, "abc123")
	}
	if loaded.Tasks[0].CompletedAt == nil {
		t.Fatal("Tasks[0].CompletedAt should not be nil")
	}
	if !loaded.Tasks[0].CompletedAt.Equal(completedAt) {
		t.Errorf("Tasks[0].CompletedAt = %v, want %v", loaded.Tasks[0].CompletedAt, completedAt)
	}
	if loaded.Tasks[1].Retries != 2 {
		t.Errorf("Tasks[1].Retries = %d, want 2", loaded.Tasks[1].Retries)
	}
	if len(loaded.Tasks[1].DependsOn) != 1 || loaded.Tasks[1].DependsOn[0] != "task-001" {
		t.Errorf("Tasks[1].DependsOn = %v, want [task-001]", loaded.Tasks[1].DependsOn)
	}
	if loaded.Tasks[2].CancelledReason != "No longer needed" {
		t.Errorf("Tasks[2].CancelledReason = %q, want %q", loaded.Tasks[2].CancelledReason, "No longer needed")
	}

	// Verify the JSON on disk is valid and has expected shape
	data, _ := os.ReadFile(filepath.Join(ForgeDir(root), stateFileName))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("saved JSON is not valid: %v", err)
	}
	if _, ok := raw["tasks"]; !ok {
		t.Error("saved JSON should have 'tasks' field")
	}
	if _, ok := raw["plan_version"]; !ok {
		t.Error("saved JSON should have 'plan_version' field")
	}
}

func TestInitForgeDir(t *testing.T) {
	root := t.TempDir()

	s, err := InitForgeDir(root)
	if err != nil {
		t.Fatalf("InitForgeDir() error: %v", err)
	}

	if s.Phase != PhasePlanning {
		t.Errorf("Phase = %q, want %q", s.Phase, PhasePlanning)
	}

	// Verify .forge/state.json was created
	statePath := filepath.Join(ForgeDir(root), stateFileName)
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("state.json not created: %v", err)
	}

	// Verify .forge/.gitignore was created with correct content
	gitignorePath := filepath.Join(ForgeDir(root), ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if string(data) != "logs/\n" {
		t.Errorf(".gitignore content = %q, want %q", string(data), "logs/\n")
	}

	// Verify .forge/logs/ was created
	logsDir := filepath.Join(ForgeDir(root), logsDirName)
	info, err := os.Stat(logsDir)
	if err != nil {
		t.Fatalf("logs directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("logs should be a directory")
	}
}

func TestProjectSnapshotRoundTrip(t *testing.T) {
	root := t.TempDir()

	original := &State{
		Phase: PhasePlanning,
		Snapshot: &ProjectSnapshot{
			IsExisting:    true,
			Language:      "Go",
			Frameworks:    []string{"gin", "gorm"},
			Dependencies:  []string{"github.com/gin-gonic/gin"},
			FileCount:     47,
			LOC:           3200,
			Structure:     "cmd/\ninternal/\ngo.mod",
			ReadmeContent: "# My Project",
			GitBranch:     "main",
			GitDirty:      false,
			RecentCommits: []string{"abc123 Initial commit"},
			KeyFiles:      []string{"Dockerfile", "Makefile"},
		},
		CreatedAt: time.Now(),
	}

	if err := Save(root, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}
	snap := loaded.Snapshot
	if !snap.IsExisting {
		t.Error("IsExisting should be true")
	}
	if snap.Language != "Go" {
		t.Errorf("Language = %q, want %q", snap.Language, "Go")
	}
	if len(snap.Frameworks) != 2 {
		t.Errorf("Frameworks length = %d, want 2", len(snap.Frameworks))
	}
	if snap.FileCount != 47 {
		t.Errorf("FileCount = %d, want 47", snap.FileCount)
	}
	if snap.LOC != 3200 {
		t.Errorf("LOC = %d, want 3200", snap.LOC)
	}
	if snap.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", snap.GitBranch, "main")
	}
	if len(snap.RecentCommits) != 1 {
		t.Errorf("RecentCommits length = %d, want 1", len(snap.RecentCommits))
	}
	if len(snap.KeyFiles) != 2 {
		t.Errorf("KeyFiles length = %d, want 2", len(snap.KeyFiles))
	}
}

func TestLogDir(t *testing.T) {
	root := t.TempDir()

	dir, err := LogDir(root)
	if err != nil {
		t.Fatalf("LogDir() error: %v", err)
	}

	expected := filepath.Join(ForgeDir(root), logsDirName)
	if dir != expected {
		t.Errorf("LogDir() = %q, want %q", dir, expected)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("logs directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("logs should be a directory")
	}
}
