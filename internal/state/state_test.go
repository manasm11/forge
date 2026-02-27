package state

import (
	"os"
	"path/filepath"
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
			Repo:        "user/repo",
			Plan: &Plan{
				ProjectName: "test-project",
				Description: "A test project",
				TechStack:   []string{"Go", "PostgreSQL"},
			},
			Settings: &Settings{
				TestCommand:   "go test ./...",
				BranchPattern: "forge/{{number}}-{{slug}}",
				MaxRetries:    3,
				AutoPR:        true,
			},
			Issues: []Issue{
				{
					SortOrder:          1,
					Title:              "Setup project",
					Description:        "Initialize the project",
					AcceptanceCriteria: []string{"go build passes"},
					Complexity:         "low",
					Status:             StatusPending,
				},
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
		if loaded.Repo != original.Repo {
			t.Errorf("Repo = %q, want %q", loaded.Repo, original.Repo)
		}
		if loaded.Plan == nil {
			t.Fatal("Plan should not be nil")
		}
		if loaded.Plan.ProjectName != original.Plan.ProjectName {
			t.Errorf("Plan.ProjectName = %q, want %q", loaded.Plan.ProjectName, original.Plan.ProjectName)
		}
		if len(loaded.Plan.TechStack) != len(original.Plan.TechStack) {
			t.Errorf("Plan.TechStack length = %d, want %d", len(loaded.Plan.TechStack), len(original.Plan.TechStack))
		}
		if loaded.Settings == nil {
			t.Fatal("Settings should not be nil")
		}
		if loaded.Settings.TestCommand != original.Settings.TestCommand {
			t.Errorf("Settings.TestCommand = %q, want %q", loaded.Settings.TestCommand, original.Settings.TestCommand)
		}
		if loaded.Settings.MaxRetries != original.Settings.MaxRetries {
			t.Errorf("Settings.MaxRetries = %d, want %d", loaded.Settings.MaxRetries, original.Settings.MaxRetries)
		}
		if loaded.Settings.AutoPR != original.Settings.AutoPR {
			t.Errorf("Settings.AutoPR = %v, want %v", loaded.Settings.AutoPR, original.Settings.AutoPR)
		}
		if len(loaded.Issues) != 1 {
			t.Fatalf("Issues length = %d, want 1", len(loaded.Issues))
		}
		if loaded.Issues[0].Title != original.Issues[0].Title {
			t.Errorf("Issues[0].Title = %q, want %q", loaded.Issues[0].Title, original.Issues[0].Title)
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

func TestRoundTrip(t *testing.T) {
	root := t.TempDir()

	completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	original := &State{
		ProjectName: "round-trip-test",
		Phase:       PhaseExecution,
		Repo:        "org/project",
		Plan: &Plan{
			ProjectName: "round-trip-test",
			Description: "Testing round trip serialization",
			TechStack:   []string{"Go", "React", "PostgreSQL"},
			RawJSON:     `{"key": "value"}`,
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
		Issues: []Issue{
			{
				SortOrder:          1,
				GitHubNumber:       42,
				Title:              "First issue",
				Description:        "Do the first thing",
				AcceptanceCriteria: []string{"criterion 1", "criterion 2"},
				DependsOn:          nil,
				Complexity:         "low",
				Status:             StatusDone,
				Branch:             "forge/1-first-issue",
				CompletedAt:        &completedAt,
			},
			{
				SortOrder:          2,
				GitHubNumber:       43,
				Title:              "Second issue",
				Description:        "Do the second thing",
				AcceptanceCriteria: []string{"criterion A"},
				DependsOn:          []int{1},
				Complexity:         "high",
				Status:             StatusInProgress,
				Retries:            2,
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
	if loaded.Repo != original.Repo {
		t.Errorf("Repo mismatch: got %q, want %q", loaded.Repo, original.Repo)
	}
	if loaded.Plan.RawJSON != original.Plan.RawJSON {
		t.Errorf("Plan.RawJSON mismatch: got %q, want %q", loaded.Plan.RawJSON, original.Plan.RawJSON)
	}
	if loaded.Settings.ExtraContext != original.Settings.ExtraContext {
		t.Errorf("Settings.ExtraContext mismatch: got %q, want %q", loaded.Settings.ExtraContext, original.Settings.ExtraContext)
	}
	if loaded.Settings.EnvVars["GO_ENV"] != "test" {
		t.Errorf("Settings.EnvVars[GO_ENV] = %q, want %q", loaded.Settings.EnvVars["GO_ENV"], "test")
	}
	if len(loaded.Issues) != 2 {
		t.Fatalf("Issues length = %d, want 2", len(loaded.Issues))
	}
	if loaded.Issues[0].CompletedAt == nil {
		t.Fatal("Issues[0].CompletedAt should not be nil")
	}
	if !loaded.Issues[0].CompletedAt.Equal(completedAt) {
		t.Errorf("Issues[0].CompletedAt = %v, want %v", loaded.Issues[0].CompletedAt, completedAt)
	}
	if loaded.Issues[1].Retries != 2 {
		t.Errorf("Issues[1].Retries = %d, want 2", loaded.Issues[1].Retries)
	}
	if len(loaded.Issues[1].DependsOn) != 1 || loaded.Issues[1].DependsOn[0] != 1 {
		t.Errorf("Issues[1].DependsOn = %v, want [1]", loaded.Issues[1].DependsOn)
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
