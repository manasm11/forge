package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Phase string

const (
	PhasePlanning  Phase = "planning"
	PhaseReview    Phase = "review"
	PhaseInputs    Phase = "inputs"
	PhaseExecution Phase = "execution"
	PhaseDone      Phase = "done"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in-progress"
	TaskDone       TaskStatus = "done"
	TaskFailed     TaskStatus = "failed"
	TaskSkipped    TaskStatus = "skipped"
	TaskCancelled  TaskStatus = "cancelled"
)

type State struct {
	ProjectName         string            `json:"project_name,omitempty"`
	Phase               Phase             `json:"phase"`
	PlanVersion         int               `json:"plan_version"`
	PlanHistory         []PlanRevision    `json:"plan_history,omitempty"`
	ConversationHistory []ConversationMsg `json:"conversation_history,omitempty"`
	Tasks               []Task            `json:"tasks,omitempty"`
	Settings            *Settings         `json:"settings,omitempty"`
	Snapshot            *ProjectSnapshot  `json:"snapshot,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// ProjectSnapshot holds detected project context for the planning phase.
type ProjectSnapshot struct {
	IsExisting    bool     `json:"is_existing"`
	Language      string   `json:"language,omitempty"`
	Frameworks    []string `json:"frameworks,omitempty"`
	Dependencies  []string `json:"dependencies,omitempty"`
	FileCount     int      `json:"file_count"`
	LOC           int      `json:"loc_estimate"`
	Structure     string   `json:"structure"`
	ReadmeContent string   `json:"readme,omitempty"`
	ClaudeMD      string   `json:"claude_md,omitempty"`
	GitBranch     string   `json:"git_branch,omitempty"`
	GitDirty      bool     `json:"git_dirty"`
	RecentCommits []string `json:"recent_commits,omitempty"`
	KeyFiles      []string `json:"key_files,omitempty"`
}

// PlanRevision records metadata each time the plan changes.
type PlanRevision struct {
	Version   int       `json:"version"`
	Summary   string    `json:"summary"`
	Timestamp time.Time `json:"timestamp"`
}

// ConversationMsg stores the planning conversation for context continuity.
// When the user replans, Claude sees this full history.
type ConversationMsg struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

type Task struct {
	ID                  string     `json:"id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	AcceptanceCriteria  []string   `json:"acceptance_criteria"`
	DependsOn           []string   `json:"depends_on,omitempty"`
	Complexity          string     `json:"complexity"`
	Status              TaskStatus `json:"status"`
	PlanVersionCreated  int        `json:"plan_version_created"`
	PlanVersionModified int        `json:"plan_version_modified"`
	Branch              string     `json:"branch,omitempty"`
	GitSHA              string     `json:"git_sha,omitempty"`
	CancelledReason     string     `json:"cancelled_reason,omitempty"`
	Retries             int        `json:"retries"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

type Settings struct {
	TestCommand   string            `json:"test_command,omitempty"`
	BuildCommand  string            `json:"build_command,omitempty"`
	BranchPattern string            `json:"branch_pattern"`
	MaxRetries    int               `json:"max_retries"`
	AutoPR        bool              `json:"auto_pr"`
	ClaudeModel   string            `json:"claude_model,omitempty"`
	MaxTurns      MaxTurnsConfig    `json:"max_turns"`
	MCPServers    []MCPServerConfig `json:"mcp_servers,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	ExtraContext  string            `json:"extra_context,omitempty"`
}

// MaxTurnsConfig maps task complexity to max claude turns.
type MaxTurnsConfig struct {
	Small  int `json:"small"`
	Medium int `json:"medium"`
	Large  int `json:"large"`
}

// MCPServerConfig stores a configured MCP server.
type MCPServerConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

const forgeDirName = ".forge"
const stateFileName = "state.json"
const logsDirName = "logs"

// ForgeDir returns the .forge directory path relative to the given project root.
func ForgeDir(root string) string {
	return filepath.Join(root, forgeDirName)
}

// Load reads state from .forge/state.json. Returns nil, nil if no state file exists.
func Load(root string) (*State, error) {
	path := filepath.Join(ForgeDir(root), stateFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	return &s, nil
}

// Save writes state to .forge/state.json. Creates .forge/ dir if needed.
// Always updates UpdatedAt before writing.
func Save(root string, s *State) error {
	dir := ForgeDir(root)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating .forge directory: %w", err)
	}

	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	path := filepath.Join(dir, stateFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// Init creates a new default state and saves it. Errors if state already exists.
func Init(root string) (*State, error) {
	path := filepath.Join(ForgeDir(root), stateFileName)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("state already exists at %s", path)
	}

	now := time.Now()
	s := &State{
		Phase:     PhasePlanning,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := Save(root, s); err != nil {
		return nil, err
	}

	return s, nil
}

// LogDir returns the .forge/logs/ directory path, creating it if needed.
func LogDir(root string) (string, error) {
	dir := filepath.Join(ForgeDir(root), logsDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating logs directory: %w", err)
	}
	return dir, nil
}

// NextTaskID generates the next sequential task ID based on existing tasks.
// Returns "task-001" if no tasks exist, "task-004" if 3 tasks exist, etc.
func (s *State) NextTaskID() string {
	maxNum := 0
	for _, t := range s.Tasks {
		// Parse the number from "task-NNN"
		parts := strings.SplitN(t.ID, "-", 2)
		if len(parts) == 2 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	return fmt.Sprintf("task-%03d", maxNum+1)
}

// AddTask appends a task with the next available ID and current plan version.
func (s *State) AddTask(title, description, complexity string, criteria []string, dependsOn []string) *Task {
	task := Task{
		ID:                  s.NextTaskID(),
		Title:               title,
		Description:         description,
		Complexity:          complexity,
		AcceptanceCriteria:  criteria,
		DependsOn:           dependsOn,
		Status:              TaskPending,
		PlanVersionCreated:  s.PlanVersion,
		PlanVersionModified: s.PlanVersion,
	}
	s.Tasks = append(s.Tasks, task)
	return &s.Tasks[len(s.Tasks)-1]
}

// PendingTasks returns tasks with status "pending" in order.
func (s *State) PendingTasks() []Task {
	var result []Task
	for _, t := range s.Tasks {
		if t.Status == TaskPending {
			result = append(result, t)
		}
	}
	return result
}

// CompletedTasks returns tasks with status "done" in order.
func (s *State) CompletedTasks() []Task {
	var result []Task
	for _, t := range s.Tasks {
		if t.Status == TaskDone {
			result = append(result, t)
		}
	}
	return result
}

// FailedTasks returns tasks with status "failed".
func (s *State) FailedTasks() []Task {
	var result []Task
	for _, t := range s.Tasks {
		if t.Status == TaskFailed {
			result = append(result, t)
		}
	}
	return result
}

// ActiveTasks returns tasks that are not cancelled or skipped.
func (s *State) ActiveTasks() []Task {
	var result []Task
	for _, t := range s.Tasks {
		if t.Status != TaskCancelled && t.Status != TaskSkipped {
			result = append(result, t)
		}
	}
	return result
}

// FindTask returns a pointer to the task with the given ID, or nil.
func (s *State) FindTask(id string) *Task {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i]
		}
	}
	return nil
}

// CancelTask marks a task as cancelled with a reason. Only pending tasks can be cancelled.
// Returns an error if the task is not found or is already done/in-progress.
func (s *State) CancelTask(id string, reason string) error {
	t := s.FindTask(id)
	if t == nil {
		return fmt.Errorf("task %q not found", id)
	}
	switch t.Status {
	case TaskDone:
		return fmt.Errorf("cannot cancel task %q: already done", id)
	case TaskInProgress:
		return fmt.Errorf("cannot cancel task %q: currently in progress", id)
	case TaskCancelled:
		return fmt.Errorf("cannot cancel task %q: already cancelled", id)
	}
	t.Status = TaskCancelled
	t.CancelledReason = reason
	return nil
}

// BumpPlanVersion increments PlanVersion, records a PlanRevision, and returns the new version.
func (s *State) BumpPlanVersion(summary string) int {
	s.PlanVersion++
	s.PlanHistory = append(s.PlanHistory, PlanRevision{
		Version:   s.PlanVersion,
		Summary:   summary,
		Timestamp: time.Now(),
	})
	return s.PlanVersion
}

// AddConversationMessage appends to conversation history.
// If history exceeds 50 messages, trims the oldest 20 into a summary.
func (s *State) AddConversationMessage(role, content string) {
	s.ConversationHistory = append(s.ConversationHistory, ConversationMsg{
		Role:    role,
		Content: content,
	})
	if len(s.ConversationHistory) > 50 {
		s.TrimConversationHistory(30)
	}
}

// TrimConversationHistory keeps the last maxMessages messages.
// Older messages are summarized into a single system message at the start.
func (s *State) TrimConversationHistory(maxMessages int) {
	if len(s.ConversationHistory) <= maxMessages {
		return
	}
	trimCount := len(s.ConversationHistory) - maxMessages
	summary := ConversationMsg{
		Role:    "system",
		Content: fmt.Sprintf("[Earlier conversation truncated — %d messages removed]", trimCount),
	}
	remaining := make([]ConversationMsg, 0, maxMessages+1)
	remaining = append(remaining, summary)
	remaining = append(remaining, s.ConversationHistory[trimCount:]...)
	s.ConversationHistory = remaining
}

// ExecutableTasks returns pending tasks whose dependencies are all done.
// Tasks whose dependencies include a failed, cancelled, or skipped task are automatically skipped.
// This cascades: if A fails, B (depends on A) is skipped, and C (depends on B) is also skipped.
func (s *State) ExecutableTasks() []Task {
	// Build a status map for quick lookup
	statusMap := make(map[string]TaskStatus, len(s.Tasks))
	for _, t := range s.Tasks {
		statusMap[t.ID] = t.Status
	}

	// Skip tasks with blocked dependencies. Loop until stable since skips cascade.
	changed := true
	for changed {
		changed = false
		for i := range s.Tasks {
			if s.Tasks[i].Status != TaskPending {
				continue
			}
			for _, dep := range s.Tasks[i].DependsOn {
				depStatus := statusMap[dep]
				if depStatus == TaskFailed || depStatus == TaskCancelled || depStatus == TaskSkipped {
					s.Tasks[i].Status = TaskSkipped
					statusMap[s.Tasks[i].ID] = TaskSkipped
					changed = true
					break
				}
			}
		}
	}

	var result []Task
	for _, t := range s.Tasks {
		if t.Status != TaskPending {
			continue
		}
		allDepsDone := true
		for _, dep := range t.DependsOn {
			if statusMap[dep] != TaskDone {
				allDepsDone = false
				break
			}
		}
		if allDepsDone {
			result = append(result, t)
		}
	}
	return result
}

// InitForgeDir creates the .forge directory structure and its .gitignore.
// Creates: .forge/, .forge/.gitignore (ignoring logs/), .forge/logs/, .forge/state.json
func InitForgeDir(root string) (*State, error) {
	dir := ForgeDir(root)

	// Create .forge/ and .forge/logs/
	logsDir := filepath.Join(dir, logsDirName)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .forge/logs directory: %w", err)
	}

	// Create .forge/.gitignore
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("logs/\n"), 0644); err != nil {
		return nil, fmt.Errorf("creating .forge/.gitignore: %w", err)
	}

	now := time.Now()
	s := &State{
		Phase:     PhasePlanning,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := Save(root, s); err != nil {
		return nil, err
	}

	return s, nil
}

// GenerateReplanContext builds the system context string that gets injected
// when the user goes back to planning.
func (s *State) GenerateReplanContext() string {
	var b strings.Builder

	b.WriteString("[System context — current project state]\n")
	fmt.Fprintf(&b, "Plan version: %d\n", s.PlanVersion)
	if s.ProjectName != "" {
		fmt.Fprintf(&b, "Project: %s\n", s.ProjectName)
	}

	completed := s.CompletedTasks()
	if len(completed) > 0 {
		b.WriteString("\nCOMPLETED TASKS (do NOT regenerate or modify these):\n")
		for _, t := range completed {
			fmt.Fprintf(&b, "  %s: %s\n", t.ID, t.Title)
		}
	}

	pending := s.PendingTasks()
	if len(pending) > 0 {
		b.WriteString("\nPENDING TASKS (can be modified, reordered, or removed):\n")
		for _, t := range pending {
			fmt.Fprintf(&b, "  %s: %s\n", t.ID, t.Title)
		}
	}

	failed := s.FailedTasks()
	if len(failed) > 0 {
		b.WriteString("\nFAILED TASKS (may need to be retried or redesigned):\n")
		for _, t := range failed {
			detail := t.Title
			if t.Retries > 0 {
				detail += fmt.Sprintf(" (failed after %d retries)", t.Retries)
			}
			fmt.Fprintf(&b, "  %s: %s\n", t.ID, detail)
		}
	}

	var cancelled []Task
	for _, t := range s.Tasks {
		if t.Status == TaskCancelled {
			cancelled = append(cancelled, t)
		}
	}
	if len(cancelled) > 0 {
		b.WriteString("\nCANCELLED TASKS:\n")
		for _, t := range cancelled {
			detail := t.Title
			if t.CancelledReason != "" {
				detail += fmt.Sprintf(" (%s)", t.CancelledReason)
			}
			fmt.Fprintf(&b, "  %s: %s\n", t.ID, detail)
		}
	}

	b.WriteString("\nWhen generating the updated plan, you MUST:\n")
	b.WriteString("- Keep all completed tasks exactly as they are — do not regenerate them\n")
	b.WriteString("- You may modify, remove, or reorder pending tasks\n")
	b.WriteString("- You may add new tasks\n")
	b.WriteString("- For failed tasks, you may redesign them as new tasks\n")
	b.WriteString("- Output the updated plan inside <plan_update> tags with the JSON format specified\n")

	return b.String()
}
