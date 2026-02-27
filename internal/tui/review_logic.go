package tui

import (
	"fmt"
	"strings"

	"github.com/manasm11/forge/internal/state"
)

// TaskDisplayItem represents a task as shown in the review list.
// Pre-computed from state.Task for rendering.
type TaskDisplayItem struct {
	ID         string
	Title      string
	Complexity string
	Status     state.TaskStatus
	DependsOn  []string
	Editable   bool // false for done/cancelled/in-progress tasks
	Index      int  // position in the display list
}

// TaskStats returns counts for display: total, done, pending, failed, cancelled.
type TaskStats struct {
	Total     int
	Done      int
	Pending   int
	Failed    int
	Cancelled int
}

// BuildTaskDisplayList converts state tasks into display items.
// Filters out cancelled tasks (they're hidden from review).
// Orders by: done tasks first (in completion order), then pending tasks (in sort order).
func BuildTaskDisplayList(tasks []state.Task) []TaskDisplayItem {
	var done, rest []state.Task
	for _, t := range tasks {
		if t.Status == state.TaskCancelled {
			continue
		}
		if t.Status == state.TaskDone {
			done = append(done, t)
		} else {
			rest = append(rest, t)
		}
	}

	var items []TaskDisplayItem
	idx := 0
	for _, t := range done {
		items = append(items, TaskDisplayItem{
			ID:         t.ID,
			Title:      t.Title,
			Complexity: t.Complexity,
			Status:     t.Status,
			DependsOn:  t.DependsOn,
			Editable:   false,
			Index:      idx,
		})
		idx++
	}
	for _, t := range rest {
		editable := t.Status == state.TaskPending || t.Status == state.TaskFailed
		items = append(items, TaskDisplayItem{
			ID:         t.ID,
			Title:      t.Title,
			Complexity: t.Complexity,
			Status:     t.Status,
			DependsOn:  t.DependsOn,
			Editable:   editable,
			Index:      idx,
		})
		idx++
	}

	return items
}

// ReorderTask moves a task in the given direction among pending tasks.
// Only pending tasks can be reordered. Done tasks are pinned at the top.
// direction: -1 = up, +1 = down.
// Returns the updated full task slice (does not mutate input).
func ReorderTask(tasks []state.Task, taskID string, direction int) ([]state.Task, error) {
	// Find the task index
	taskIdx := -1
	for i, t := range tasks {
		if t.ID == taskID {
			taskIdx = i
			break
		}
	}
	if taskIdx == -1 {
		return nil, fmt.Errorf("task %q not found", taskID)
	}

	if tasks[taskIdx].Status != state.TaskPending {
		return nil, fmt.Errorf("only pending tasks can be reordered")
	}

	// Find the adjacent pending task in the given direction
	swapIdx := -1
	if direction < 0 {
		// Moving up: find the nearest pending task before this one
		for i := taskIdx - 1; i >= 0; i-- {
			if tasks[i].Status == state.TaskPending {
				swapIdx = i
				break
			}
		}
	} else {
		// Moving down: find the nearest pending task after this one
		for i := taskIdx + 1; i < len(tasks); i++ {
			if tasks[i].Status == state.TaskPending {
				swapIdx = i
				break
			}
		}
	}

	if swapIdx == -1 {
		return nil, fmt.Errorf("cannot move task further in that direction")
	}

	// Create a copy and swap
	result := make([]state.Task, len(tasks))
	copy(result, tasks)
	result[taskIdx], result[swapIdx] = result[swapIdx], result[taskIdx]

	return result, nil
}

// DeleteTask removes a pending task from the slice.
// Returns error if task is done/in-progress or not found.
// Also removes this task's ID from any other task's DependsOn list.
// Does not mutate the input slice.
func DeleteTask(tasks []state.Task, taskID string) ([]state.Task, error) {
	// Find the task
	found := false
	for _, t := range tasks {
		if t.ID == taskID {
			found = true
			if t.Status == state.TaskDone {
				return nil, fmt.Errorf("cannot delete completed task %q", taskID)
			}
			if t.Status == state.TaskInProgress {
				return nil, fmt.Errorf("cannot delete in-progress task %q", taskID)
			}
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("task %q not found", taskID)
	}

	// Build new slice without the deleted task, cleaning up dependencies
	var result []state.Task
	for _, t := range tasks {
		if t.ID == taskID {
			continue
		}
		// Copy the task and clean up DependsOn
		newTask := t
		if len(t.DependsOn) > 0 {
			var cleanedDeps []string
			for _, dep := range t.DependsOn {
				if dep != taskID {
					cleanedDeps = append(cleanedDeps, dep)
				}
			}
			newTask.DependsOn = cleanedDeps
		}
		result = append(result, newTask)
	}

	return result, nil
}

// ValidateNewTask checks that a manually added task has valid fields.
// Title must be non-empty. Complexity must be small/medium/large.
// DependsOn IDs must reference existing tasks.
func ValidateNewTask(tasks []state.Task, title, description, complexity string, criteria []string, dependsOn []string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title must not be empty")
	}

	switch complexity {
	case "small", "medium", "large":
		// valid
	default:
		return fmt.Errorf("complexity must be small, medium, or large (got %q)", complexity)
	}

	// Check that all dependencies exist
	taskIDs := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		taskIDs[t.ID] = true
	}
	for _, dep := range dependsOn {
		if !taskIDs[dep] {
			return fmt.Errorf("dependency %q does not exist", dep)
		}
	}

	return nil
}

// FormatTaskDetail produces the expanded detail text for a task.
// Includes: title, complexity, dependencies (resolved to titles), description, acceptance criteria.
func FormatTaskDetail(task state.Task, allTasks []state.Task) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s: %s\n", task.ID, task.Title)
	fmt.Fprintf(&b, "Complexity: %s", task.Complexity)

	if len(task.DependsOn) > 0 {
		depTitles := ResolveDependencyTitles(task.DependsOn, allTasks)
		fmt.Fprintf(&b, " · Depends on: %s", strings.Join(depTitles, ", "))
	}
	b.WriteString("\n")

	if task.Description != "" {
		fmt.Fprintf(&b, "%s\n", task.Description)
	}

	if len(task.AcceptanceCriteria) > 0 {
		b.WriteString("Acceptance Criteria:\n")
		for _, c := range task.AcceptanceCriteria {
			fmt.Fprintf(&b, "• %s\n", c)
		}
	}

	return b.String()
}

// ResolveDependencyTitles maps task IDs in DependsOn to their titles.
// Returns "task-001: Init project" format. Unknown IDs return "task-XXX: (unknown)".
func ResolveDependencyTitles(dependsOn []string, allTasks []state.Task) []string {
	titleMap := make(map[string]string, len(allTasks))
	for _, t := range allTasks {
		titleMap[t.ID] = t.Title
	}

	var result []string
	for _, id := range dependsOn {
		title, ok := titleMap[id]
		if !ok {
			result = append(result, fmt.Sprintf("%s: (unknown)", id))
		} else {
			result = append(result, fmt.Sprintf("%s: %s", id, title))
		}
	}
	return result
}

// ComputeTaskStats returns counts for display: total, done, pending, failed, cancelled.
func ComputeTaskStats(tasks []state.Task) TaskStats {
	var stats TaskStats
	stats.Total = len(tasks)
	for _, t := range tasks {
		switch t.Status {
		case state.TaskDone:
			stats.Done++
		case state.TaskPending:
			stats.Pending++
		case state.TaskFailed:
			stats.Failed++
		case state.TaskCancelled:
			stats.Cancelled++
		}
	}
	return stats
}

// CanConfirm checks if the task list is valid for proceeding to execution.
// Returns an error message if not (e.g., no pending tasks, circular dependencies).
// Returns "" if valid.
func CanConfirm(tasks []state.Task) string {
	// Check at least one pending task
	hasPending := false
	for _, t := range tasks {
		if t.Status == state.TaskPending {
			hasPending = true
			break
		}
	}
	if !hasPending {
		return "no pending tasks to execute"
	}

	// Check for circular dependencies
	if cycle := DetectCircularDependencies(tasks); len(cycle) > 0 {
		return fmt.Sprintf("circular dependency detected: %s", strings.Join(cycle, " → "))
	}

	return ""
}

// DetectCircularDependencies checks for cycles in the task dependency graph.
// Only considers pending tasks — done/cancelled tasks are treated as resolved.
// Returns the IDs involved in the cycle, or nil if no cycles.
func DetectCircularDependencies(tasks []state.Task) []string {
	// Build adjacency list for pending tasks only
	pending := make(map[string]bool)
	deps := make(map[string][]string)
	for _, t := range tasks {
		if t.Status == state.TaskPending {
			pending[t.ID] = true
			// Only include dependencies that are also pending
			for _, dep := range t.DependsOn {
				for _, other := range tasks {
					if other.ID == dep && other.Status == state.TaskPending {
						deps[t.ID] = append(deps[t.ID], dep)
						break
					}
				}
			}
		}
	}

	if len(pending) == 0 {
		return nil
	}

	// DFS with coloring: 0=white (unvisited), 1=gray (in stack), 2=black (done)
	color := make(map[string]int)
	parent := make(map[string]string)

	var cycleNodes []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = 1 // gray
		for _, dep := range deps[node] {
			if color[dep] == 1 {
				// Found a cycle - trace it back
				cycleNodes = []string{dep, node}
				cur := node
				for cur != dep {
					cur = parent[cur]
					if cur == "" || cur == dep {
						break
					}
					cycleNodes = append(cycleNodes, cur)
				}
				return true
			}
			if color[dep] == 0 {
				parent[dep] = node
				if dfs(dep) {
					return true
				}
			}
		}
		color[node] = 2 // black
		return false
	}

	for id := range pending {
		if color[id] == 0 {
			if dfs(id) {
				return cycleNodes
			}
		}
	}

	return nil
}
