package preflight

import (
	"testing"
)

func TestRunAll_ReturnsAllChecks(t *testing.T) {
	t.Parallel()
	results := RunAll()

	names := make(map[string]bool)
	for _, r := range results {
		names[r.Name] = true
	}

	required := []string{"git", "claude", "gh"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("missing check for %q", name)
		}
	}
}

func TestRunAll_DetectsGit(t *testing.T) {
	t.Parallel()
	results := RunAll()

	var gitResult *CheckResult
	for i := range results {
		if results[i].Name == "git" {
			gitResult = &results[i]
			break
		}
	}

	if gitResult == nil {
		t.Fatal("git check should be in results")
	}
	if !gitResult.Found {
		t.Skip("git not installed in test environment")
	}
	if gitResult.Version == "" {
		t.Error("git version should not be empty")
	}
}

func TestRunAll_ResultCount(t *testing.T) {
	t.Parallel()
	results := RunAll()

	if len(results) != 3 {
		t.Errorf("RunAll() returned %d results, want 3", len(results))
	}
}

func TestCheck_NonExistentTool(t *testing.T) {
	t.Parallel()
	result := check("nonexistent_xyz_abc_tool")

	if result.Found {
		t.Error("non-existent tool should have Found=false")
	}
	if result.Error == "" {
		t.Error("non-existent tool should have non-empty Error")
	}
	if result.Name != "nonexistent_xyz_abc_tool" {
		t.Errorf("Name = %q, want %q", result.Name, "nonexistent_xyz_abc_tool")
	}
}

func TestCheck_Git(t *testing.T) {
	t.Parallel()
	result := check("git")

	if !result.Found {
		t.Skip("git not installed in test environment")
	}
	if result.Version == "" {
		t.Error("git Version should not be empty when Found=true")
	}
	if result.Name != "git" {
		t.Errorf("Name = %q, want %q", result.Name, "git")
	}
}

func TestCheckResult_FieldsPopulated(t *testing.T) {
	t.Parallel()
	results := RunAll()

	for _, r := range results {
		if r.Name == "" {
			t.Error("CheckResult.Name should not be empty")
		}
		if !r.Found && r.Error == "" {
			t.Errorf("CheckResult for %q: Found=false but Error is empty", r.Name)
		}
	}
}
