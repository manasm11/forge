package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manasm11/forge/internal/provider"
	"github.com/manasm11/forge/internal/state"
)

// Runner orchestrates task execution.
type Runner struct {
	cfg RunnerConfig
}

// NewRunner creates a new execution runner.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{cfg: cfg}
}

// Run executes all pending tasks in dependency order.
// Returns when all tasks are done, failed, or skipped.
// Can be cancelled via context.
func (r *Runner) Run(ctx context.Context) error {
	baseBranch := r.cfg.BaseBranch
	if baseBranch == "" {
		var err error
		baseBranch, err = r.cfg.Git.CurrentBranch(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
	}

	// Track completed task branches for merging
	var completedBranches []string

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// ExecutableTasks handles skipping tasks with failed/cancelled deps
		executable := r.cfg.State.ExecutableTasks()
		if len(executable) == 0 {
			break
		}

		// Find the actual task in state (not the copy from ExecutableTasks)
		stateTask := r.cfg.State.FindTask(executable[0].ID)
		if stateTask == nil {
			break
		}

		outcome := r.RunTask(ctx, stateTask)

		// Update state
		stateTask.Status = outcome.Status
		if outcome.Status == state.TaskDone {
			now := time.Now()
			stateTask.CompletedAt = &now
			stateTask.GitSHA = outcome.SHA
			// Track branch for merging
			if stateTask.Branch != "" {
				completedBranches = append(completedBranches, stateTask.Branch)
			}
		}
		stateTask.Retries = outcome.Retries

		// Persist state after each task
		state.Save(r.cfg.StateRoot, r.cfg.State)

		// Write log file
		r.writeLog(stateTask.ID, outcome.Logs)

		// Return to base branch
		r.cfg.Git.CheckoutBranch(ctx, baseBranch)

		// Emit events for task outcome
		if outcome.Status == state.TaskDone {
			r.emit(TaskEvent{TaskID: stateTask.ID, Type: EventTaskDone, Message: "completed"})
		}
	}

	// After all tasks, handle merging/pushing
	if len(completedBranches) > 0 {
		// Merge all completed branches into base branch
		for _, branch := range completedBranches {
			if err := r.cfg.Git.Merge(ctx, branch); err != nil {
				r.emit(TaskEvent{Type: EventError, Message: fmt.Sprintf("failed to merge %s: %v", branch, err)})
			}
		}

		// Checkout base branch after merging
		r.cfg.Git.CheckoutBranch(ctx, baseBranch)

		// Push if remote exists
		if r.cfg.RemoteURL != "" {
			if err := r.cfg.Git.Push(ctx); err != nil {
				r.emit(TaskEvent{Type: EventError, Message: fmt.Sprintf("failed to push: %v", err)})
			}
		} else {
			r.emit(TaskEvent{Type: EventPush, Message: "No remote configured - skipped push"})
		}
	}

	return nil
}

// RunTask executes a single task.
func (r *Runner) RunTask(ctx context.Context, task *state.Task) TaskOutcome {
	var log strings.Builder
	settings := r.cfg.State.Settings
	branchName := ResolveBranchName(settings.BranchPattern, task.ID)
	branchName = SanitizeBranchName(branchName)
	task.Branch = branchName

	// Record base branch for returning later
	baseBranch, _ := r.cfg.Git.CurrentBranch(ctx)

	// Emit start event
	r.emit(TaskEvent{TaskID: task.ID, Type: EventTaskStart, Message: task.Title})

	// 1. Branch setup
	exists, _ := r.cfg.Git.BranchExists(ctx, branchName)
	if exists {
		if err := r.cfg.Git.CheckoutBranch(ctx, branchName); err != nil {
			return r.fail(task.ID, "checkout existing branch: "+err.Error(), &log, 0)
		}
	} else {
		if err := r.cfg.Git.CreateBranch(ctx, branchName, baseBranch); err != nil {
			return r.fail(task.ID, "create branch: "+err.Error(), &log, 0)
		}
	}
	r.emit(TaskEvent{TaskID: task.ID, Type: EventBranchCreated, Message: branchName})

	// 2. Execute with retries
	maxRetries := settings.MaxRetries
	maxAttempts := 1 + maxRetries
	var lastTestOutput string

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return r.fail(task.ID, "cancelled", &log, attempt)
		}

		// Build prompt
		var prompt string
		if attempt == 0 {
			prompt = BuildTaskExecutionPrompt(r.cfg.ContextFile, *task, settings)
		} else {
			r.emit(TaskEvent{TaskID: task.ID, Type: EventRetry,
				Message: fmt.Sprintf("Retry %d/%d", attempt, maxRetries)})
			prompt = BuildRetryPrompt(attempt, maxRetries, lastTestOutput)
		}

		// Build provider env vars
		providerEnv := provider.EnvVarsForProvider(settings.Provider)

		// Merge: settings.EnvVars + provider env vars (provider wins on collision)
		mergedEnv := provider.MergeEnvVars(settings.EnvVars, providerEnv)

		// Run Claude
		r.emit(TaskEvent{TaskID: task.ID, Type: EventClaudeStart})
		result, err := r.cfg.Claude.Execute(ctx, ExecuteOpts{
			Prompt:       prompt,
			SystemPrompt: BuildExecutionSystemPrompt(),
			Model:        settings.Provider.Model, // use provider model, not settings.ClaudeModel
			MaxTurns:     MaxTurnsForTask(task.Complexity, settings.MaxTurns),
			AllowedTools: BuildAllowedTools(settings.MCPServers),
			WorkDir:      r.cfg.StateRoot,
			EnvVars:      mergedEnv,
			OnChunk: func(text string) {
				r.emit(TaskEvent{TaskID: task.ID, Type: EventClaudeChunk, Detail: text})
			},
		})
		if err != nil {
			return r.fail(task.ID, "claude execution: "+err.Error(), &log, attempt)
		}
		log.WriteString(fmt.Sprintf("=== Claude Output (attempt %d) ===\n", attempt+1))
		log.WriteString(result.Text + "\n\n")
		r.emit(TaskEvent{TaskID: task.ID, Type: EventClaudeDone})

		// Run tests
		allPassed := true

		if settings.TestCommand != "" {
			r.emit(TaskEvent{TaskID: task.ID, Type: EventTestStart, Message: settings.TestCommand})
			testResult := r.cfg.Tests.RunTests(ctx, settings.TestCommand)
			log.WriteString("=== Test Output ===\n" + testResult.Output + "\n\n")

			if !testResult.Passed {
				allPassed = false
				lastTestOutput = testResult.Output
				r.emit(TaskEvent{TaskID: task.ID, Type: EventTestFailed, Detail: testResult.Output})
			} else {
				r.emit(TaskEvent{TaskID: task.ID, Type: EventTestPassed})
			}
		}

		// Run build if configured and tests passed
		if allPassed && settings.BuildCommand != "" {
			r.emit(TaskEvent{TaskID: task.ID, Type: EventBuildStart, Message: settings.BuildCommand})
			buildResult := r.cfg.Tests.RunBuild(ctx, settings.BuildCommand)
			log.WriteString("=== Build Output ===\n" + buildResult.Output + "\n\n")

			if !buildResult.Passed {
				allPassed = false
				lastTestOutput = buildResult.Output
				r.emit(TaskEvent{TaskID: task.ID, Type: EventBuildFailed, Detail: buildResult.Output})
			} else {
				r.emit(TaskEvent{TaskID: task.ID, Type: EventBuildPassed})
			}
		}

		if allPassed {
			// 3. Stage, commit, push
			if err := r.cfg.Git.StageAll(ctx); err != nil {
				return r.fail(task.ID, "stage: "+err.Error(), &log, attempt)
			}

			hasStagedChanges, _ := r.cfg.Git.HasStagedChanges(ctx)
			if !hasStagedChanges {
				return r.fail(task.ID, "no code changes produced", &log, attempt)
			}

			msg := CommitMessage(task.ID, task.Title)
			sha, err := r.cfg.Git.Commit(ctx, msg)
			if err != nil {
				return r.fail(task.ID, "commit: "+err.Error(), &log, attempt)
			}
			r.emit(TaskEvent{TaskID: task.ID, Type: EventCommit, Message: sha})

			if err := r.cfg.Git.Push(ctx); err != nil {
				return r.fail(task.ID, "push: "+err.Error(), &log, attempt)
			}
			r.emit(TaskEvent{TaskID: task.ID, Type: EventPush})

			// Update task state directly
			task.Status = state.TaskDone
			task.GitSHA = sha
			task.Retries = attempt
			now := time.Now()
			task.CompletedAt = &now

			// Return to base branch
			r.cfg.Git.CheckoutBranch(ctx, baseBranch)

			return TaskOutcome{
				TaskID:  task.ID,
				Status:  state.TaskDone,
				SHA:     sha,
				Retries: attempt,
				Logs:    log.String(),
			}
		}
	}

	// Exhausted retries — return to base branch
	r.cfg.Git.CheckoutBranch(ctx, baseBranch)

	r.emit(TaskEvent{TaskID: task.ID, Type: EventTaskFailed, Message: "exhausted retries"})
	return TaskOutcome{
		TaskID:  task.ID,
		Status:  state.TaskFailed,
		Error:   fmt.Sprintf("tests failed after %d attempts", maxAttempts),
		Retries: maxRetries,
		Logs:    log.String(),
	}
}

func (r *Runner) emit(event TaskEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if r.cfg.OnEvent != nil {
		r.cfg.OnEvent(event)
	}
}

func (r *Runner) fail(taskID, message string, log *strings.Builder, retries int) TaskOutcome {
	r.emit(TaskEvent{TaskID: taskID, Type: EventTaskFailed, Message: message})
	log.WriteString("=== FAILED: " + message + " ===\n")
	return TaskOutcome{
		TaskID:  taskID,
		Status:  state.TaskFailed,
		Error:   message,
		Retries: retries,
		Logs:    log.String(),
	}
}

func (r *Runner) writeLog(taskID, content string) {
	dir := filepath.Join(r.cfg.StateRoot, ".forge", "logs")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, taskID+".log")
	os.WriteFile(path, []byte(content), 0644)
}
