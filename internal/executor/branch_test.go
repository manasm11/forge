package executor

import (
	"testing"
)

func TestResolveBranchName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		taskID  string
		want    string
	}{
		{"default pattern", "forge/task-{id}", "task-003", "forge/task-task-003"},
		{"simple pattern", "forge/{id}", "task-001", "forge/task-001"},
		{"custom prefix", "feature/{id}", "task-042", "feature/task-042"},
		{"no placeholder", "forge/branch", "task-001", "forge/branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveBranchName(tt.pattern, tt.taskID)
			if got != tt.want {
				t.Errorf("ResolveBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"forge/task-001", "forge/task-001"},
		{"forge/task 001", "forge/task-001"},
		{"forge/task..001", "forge/task.001"},
		{"forge/task~001", "forge/task-001"},
		{"forge/task^001", "forge/task-001"},
		{"forge/task:001", "forge/task-001"},
		{".forge/task-001", "forge/task-001"},
		{"forge/task-001.lock", "forge/task-001"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := SanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCommitMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		taskID string
		title  string
		want   string
	}{
		{"task-001", "Initialize project", "forge: task-001 — Initialize project"},
		{"task-042", "Add authentication", "forge: task-042 — Add authentication"},
	}
	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			t.Parallel()
			got := CommitMessage(tt.taskID, tt.title)
			if got != tt.want {
				t.Errorf("CommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
