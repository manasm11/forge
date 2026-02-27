package tui

import (
	"fmt"

	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/state"
)

// ApplyInitialPlan converts a PlanJSON into tasks and updates state.
// Sets project name, creates tasks with dependency resolution, and bumps plan version.
// Returns an error if the plan is invalid.
func ApplyInitialPlan(s *state.State, plan *claude.PlanJSON) error {
	if plan.ProjectName == "" {
		return fmt.Errorf("plan is missing project_name")
	}
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan has no tasks")
	}

	s.ProjectName = plan.ProjectName

	// Build a mapping from task index to task ID for dependency resolution
	taskIDs := make([]string, len(plan.Tasks))
	for i := range plan.Tasks {
		taskIDs[i] = s.NextTaskID()
		pt := plan.Tasks[i]
		var deps []string
		for _, depIdx := range pt.DependsOn {
			if depIdx >= 0 && depIdx < len(taskIDs) {
				deps = append(deps, taskIDs[depIdx])
			}
		}
		s.AddTask(pt.Title, pt.Description, pt.Complexity, pt.AcceptanceCriteria, deps)
	}

	s.BumpPlanVersion("Initial plan")
	return nil
}

// ApplyPlanUpdate applies a PlanUpdateJSON diff to existing state tasks.
// Returns an error if any action is invalid (e.g., modifying a completed task).
func ApplyPlanUpdate(s *state.State, update *claude.PlanUpdateJSON) error {
	for _, t := range update.Tasks {
		switch t.Action {
		case "keep":
			// do nothing

		case "modify":
			task := s.FindTask(t.ID)
			if task == nil {
				return fmt.Errorf("modify: task %q not found", t.ID)
			}
			if task.Status == state.TaskDone {
				return fmt.Errorf("modify: cannot modify completed task %q", t.ID)
			}
			if t.Title != "" {
				task.Title = t.Title
			}
			if t.Description != "" {
				task.Description = t.Description
			}
			if len(t.AcceptanceCriteria) > 0 {
				task.AcceptanceCriteria = t.AcceptanceCriteria
			}
			if len(t.DependsOn) > 0 {
				task.DependsOn = t.DependsOn
			}
			if t.Complexity != "" {
				task.Complexity = t.Complexity
			}
			task.PlanVersionModified = s.PlanVersion + 1

		case "add":
			s.AddTask(t.Title, t.Description, t.Complexity, t.AcceptanceCriteria, t.DependsOn)

		case "remove":
			if t.ID == "" {
				return fmt.Errorf("remove: missing task ID")
			}
			task := s.FindTask(t.ID)
			if task == nil {
				return fmt.Errorf("remove: task %q not found", t.ID)
			}
			if task.Status == state.TaskDone {
				return fmt.Errorf("remove: cannot remove completed task %q", t.ID)
			}
			reason := t.Reason
			if reason == "" {
				reason = "Removed during replanning"
			}
			if err := s.CancelTask(t.ID, reason); err != nil {
				return fmt.Errorf("remove: %w", err)
			}

		default:
			return fmt.Errorf("unknown action %q for task %q", t.Action, t.ID)
		}
	}

	return nil
}
