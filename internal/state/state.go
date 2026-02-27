package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

type IssueStatus string

const (
	StatusPending    IssueStatus = "pending"
	StatusInProgress IssueStatus = "in-progress"
	StatusDone       IssueStatus = "done"
	StatusFailed     IssueStatus = "failed"
	StatusSkipped    IssueStatus = "skipped"
)

type State struct {
	ProjectName string    `json:"project_name"`
	Phase       Phase     `json:"phase"`
	Repo        string    `json:"repo,omitempty"`
	Plan        *Plan     `json:"plan,omitempty"`
	Settings    *Settings `json:"settings,omitempty"`
	Issues      []Issue   `json:"issues,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Plan struct {
	ProjectName string   `json:"project_name"`
	Description string   `json:"description"`
	TechStack   []string `json:"tech_stack"`
	RawJSON     string   `json:"raw_json,omitempty"`
}

type Settings struct {
	TestCommand   string            `json:"test_command,omitempty"`
	BuildCommand  string            `json:"build_command,omitempty"`
	BranchPattern string            `json:"branch_pattern"`
	MaxRetries    int               `json:"max_retries"`
	AutoPR        bool              `json:"auto_pr"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	ExtraContext  string            `json:"extra_context,omitempty"`
}

type Issue struct {
	SortOrder          int         `json:"sort_order"`
	GitHubNumber       int         `json:"github_number,omitempty"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	AcceptanceCriteria []string    `json:"acceptance_criteria"`
	DependsOn          []int       `json:"depends_on,omitempty"`
	Complexity         string      `json:"complexity"`
	Status             IssueStatus `json:"status"`
	Branch             string      `json:"branch,omitempty"`
	Retries            int         `json:"retries"`
	CompletedAt        *time.Time  `json:"completed_at,omitempty"`
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
