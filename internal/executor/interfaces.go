package executor

import (
	"context"

	"github.com/manasm11/forge/internal/state"
)

// GitOps abstracts all git operations for testability.
type GitOps interface {
	// CurrentBranch returns the current branch name.
	CurrentBranch(ctx context.Context) (string, error)

	// CreateBranch creates and checks out a new branch from the base branch.
	CreateBranch(ctx context.Context, name string, baseBranch string) error

	// CheckoutBranch switches to an existing branch.
	CheckoutBranch(ctx context.Context, name string) error

	// BranchExists checks if a branch exists locally.
	BranchExists(ctx context.Context, name string) (bool, error)

	// StageAll stages all changes (git add -A).
	StageAll(ctx context.Context) error

	// HasStagedChanges returns true if there are staged changes to commit.
	HasStagedChanges(ctx context.Context) (bool, bool, error)

	// HasUnstagedChanges returns true if there are unstaged/untracked changes.
	HasUnstagedChanges(ctx context.Context) (bool, error)

	// Commit creates a commit with the given message. Returns the SHA.
	Commit(ctx context.Context, message string) (string, error)

	// Push pushes the current branch to origin.
	Push(ctx context.Context) error

	// Merge merges a branch into the current branch.
	Merge(ctx context.Context, branch string) error

	// LatestSHA returns the HEAD commit SHA.
	LatestSHA(ctx context.Context) (string, error)

	// ResetHard resets the working tree to HEAD (undo uncommitted changes).
	ResetHard(ctx context.Context) error

	// DeleteBranch deletes a local branch. Fails if it's the current branch.
	DeleteBranch(ctx context.Context, name string) error
}

// TestRunner abstracts running test/build commands.
type TestRunner interface {
	// RunTests executes the test command and returns the result.
	RunTests(ctx context.Context, command string) *TestResult

	// RunBuild executes the build command and returns the result.
	RunBuild(ctx context.Context, command string) *TestResult
}

// TestResult holds the outcome of a test or build command.
type TestResult struct {
	Passed   bool
	Output   string  // stdout+stderr combined
	ExitCode int
	Duration float64 // seconds
}

// ClaudeExecutor abstracts Claude Code CLI for execution.
type ClaudeExecutor interface {
	// Execute runs Claude Code with the given prompt and settings.
	// onChunk is called for each streaming chunk (can be nil).
	Execute(ctx context.Context, opts ExecuteOpts) (*ExecuteResult, error)
}

// ExecuteOpts configures a single Claude Code execution call.
type ExecuteOpts struct {
	Prompt       string
	SystemPrompt string
	Model        string
	MaxTurns     int
	AllowedTools []string // --allowedTools flag
	WorkDir      string   // working directory
	EnvVars      map[string]string
	OnChunk      func(text string) // streaming callback
}

// ExecuteResult holds Claude Code's response.
type ExecuteResult struct {
	Text       string
	TurnCount  int
	TokensUsed int
	Duration   float64 // seconds
}

// TaskEvent represents something that happened during task execution.
type TaskEvent struct {
	TaskID    string
	Type      TaskEventType
	Message   string
	Detail    string // longer detail (e.g., test output, error text)
	Timestamp int64  // unix millis
}

// TaskEventType classifies execution events.
type TaskEventType int

const (
	EventTaskStart TaskEventType = iota
	EventBranchCreated
	EventClaudeStart
	EventClaudeChunk
	EventClaudeDone
	EventTestStart
	EventTestPassed
	EventTestFailed
	EventBuildStart
	EventBuildPassed
	EventBuildFailed
	EventRetry
	EventCommit
	EventPush
	EventPRCreated
	EventTaskDone
	EventTaskFailed
	EventTaskSkipped
	EventError
)

// EventHandler receives execution events for logging/display.
type EventHandler func(event TaskEvent)

// RunnerConfig holds all configuration for the execution engine.
type RunnerConfig struct {
	State       *state.State
	StateRoot   string // project root
	Git         GitOps
	Tests       TestRunner
	Claude      ClaudeExecutor
	OnEvent     EventHandler
	ContextFile string // contents of .forge/context.md
	BaseBranch  string // base branch for merging
	RemoteURL   string // remote URL (empty if no remote)
}

// TaskOutcome is the result of executing a single task.
type TaskOutcome struct {
	TaskID  string
	Status  state.TaskStatus
	SHA     string // commit SHA if successful
	Error   string // error message if failed
	Retries int    // how many retries were attempted
	Logs    string // full execution log
}
