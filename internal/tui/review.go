package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/state"
	"github.com/manasm11/forge/internal/tui/components"
)

// editorFinishedMsg is sent when $EDITOR closes.
type editorFinishedMsg struct {
	err      error
	tmpPath  string
	taskID   string // empty for "new" task
	isNew    bool
}

// clearConfirmErrMsg clears the confirmation error after a timeout.
type clearConfirmErrMsg struct{}

// ReviewModel manages the task review phase.
type ReviewModel struct {
	taskList      components.TaskListModel
	state         *state.State
	stateRoot     string
	width, height int
	confirmErr    string // shown when 'c' is pressed but CanConfirm fails
	deleteConfirm string // task ID pending delete confirmation
}

// NewReviewModel creates a new review phase model.
func NewReviewModel(s *state.State, root string) ReviewModel {
	items := buildReviewItems(s)
	taskList := components.NewTaskListModel(items)

	m := ReviewModel{
		taskList:  taskList,
		state:     s,
		stateRoot: root,
	}

	return m
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

func (m ReviewModel) Update(msg tea.Msg) (ReviewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle delete confirmation mode
		if m.deleteConfirm != "" {
			return m.handleDeleteConfirm(msg)
		}

		switch msg.String() {
		case "r":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhasePlanning}
			}

		case "c":
			errMsg := CanConfirm(m.state.Tasks)
			if errMsg != "" {
				m.confirmErr = errMsg
				return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
					return clearConfirmErrMsg{}
				})
			}
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseInputs}
			}

		case "q":
			return m, tea.Quit
		}

	case components.TaskActionMsg:
		return m.handleTaskAction(msg)

	case editorFinishedMsg:
		return m.handleEditorFinished(msg)

	case clearConfirmErrMsg:
		m.confirmErr = ""
		return m, nil
	}

	// Delegate to task list
	var cmd tea.Cmd
	m.taskList, cmd = m.taskList.Update(msg)
	return m, cmd
}

func (m ReviewModel) View() string {
	if m.width == 0 {
		return ""
	}

	// Header
	stats := ComputeTaskStats(m.state.Tasks)
	header := m.renderReviewHeader(stats)

	// Task list content
	contentHeight := m.height - 2 // header + footer
	if contentHeight < 1 {
		contentHeight = 1
	}
	m.taskList.SetSize(m.width, contentHeight)
	content := m.taskList.View()

	// Footer
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m *ReviewModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// --- Header/Footer Rendering ---

func (m ReviewModel) renderReviewHeader(stats TaskStats) string {
	info := lipgloss.NewStyle().
		Foreground(Muted).
		PaddingLeft(1).
		Render(fmt.Sprintf("Plan v%d · %d pending · %d done · %d total",
			m.state.PlanVersion, stats.Pending, stats.Done, stats.Total))

	return info
}

func (m ReviewModel) renderFooter() string {
	if m.deleteConfirm != "" {
		prompt := lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true).
			Render(fmt.Sprintf("Delete %s? (y/n)", m.deleteConfirm))
		return StatusBar.Width(m.width).Render(prompt)
	}

	if m.confirmErr != "" {
		errMsg := lipgloss.NewStyle().
			Foreground(Danger).
			Bold(true).
			Render(m.confirmErr)
		return StatusBar.Width(m.width).Render(errMsg)
	}

	help := HelpStyle.Render(
		"j/k navigate · Enter details · e edit · d delete · n new · J/K reorder · r replan · c confirm · q quit")

	return StatusBar.Width(m.width).Render(help)
}

// --- Action Handlers ---

func (m ReviewModel) handleTaskAction(msg components.TaskActionMsg) (ReviewModel, tea.Cmd) {
	switch msg.Action {
	case "edit":
		return m.startEdit(msg.TaskID)
	case "delete":
		m.deleteConfirm = msg.TaskID
		return m, nil
	case "new":
		return m.startNew()
	case "reorder_up":
		return m.reorder(msg.TaskID, -1)
	case "reorder_down":
		return m.reorder(msg.TaskID, 1)
	}
	return m, nil
}

func (m ReviewModel) handleDeleteConfirm(msg tea.KeyMsg) (ReviewModel, tea.Cmd) {
	taskID := m.deleteConfirm
	m.deleteConfirm = ""

	if msg.String() != "y" {
		return m, nil
	}

	result, err := DeleteTask(m.state.Tasks, taskID)
	if err != nil {
		m.confirmErr = err.Error()
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearConfirmErrMsg{}
		})
	}

	m.state.Tasks = result
	_ = state.Save(m.stateRoot, m.state)
	m.refreshList()
	return m, nil
}

func (m ReviewModel) reorder(taskID string, direction int) (ReviewModel, tea.Cmd) {
	result, err := ReorderTask(m.state.Tasks, taskID, direction)
	if err != nil {
		m.confirmErr = err.Error()
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearConfirmErrMsg{}
		})
	}

	m.state.Tasks = result
	_ = state.Save(m.stateRoot, m.state)
	m.refreshList()
	m.taskList.SetCursorByID(taskID)
	return m, nil
}

func (m ReviewModel) startEdit(taskID string) (ReviewModel, tea.Cmd) {
	task := m.state.FindTask(taskID)
	if task == nil {
		return m, nil
	}

	// Create temp file with task data
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("forge-edit-%s.txt", taskID))

	content := formatEditTemplate(task)
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		m.confirmErr = fmt.Sprintf("Failed to create temp file: %v", err)
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearConfirmErrMsg{}
		})
	}

	editor := getEditor()
	c := exec.Command(editor, tmpPath)

	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{
			err:     err,
			tmpPath: tmpPath,
			taskID:  taskID,
			isNew:   false,
		}
	})
}

func (m ReviewModel) startNew() (ReviewModel, tea.Cmd) {
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, "forge-new-task.txt")

	content := formatNewTemplate()
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		m.confirmErr = fmt.Sprintf("Failed to create temp file: %v", err)
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearConfirmErrMsg{}
		})
	}

	editor := getEditor()
	c := exec.Command(editor, tmpPath)

	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{
			err:     err,
			tmpPath: tmpPath,
			isNew:   true,
		}
	})
}

func (m ReviewModel) handleEditorFinished(msg editorFinishedMsg) (ReviewModel, tea.Cmd) {
	// Clean up temp file on exit
	defer os.Remove(msg.tmpPath)

	if msg.err != nil {
		m.confirmErr = fmt.Sprintf("Editor error: %v", msg.err)
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearConfirmErrMsg{}
		})
	}

	data, err := os.ReadFile(msg.tmpPath)
	if err != nil {
		m.confirmErr = fmt.Sprintf("Failed to read temp file: %v", err)
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearConfirmErrMsg{}
		})
	}

	parsed := parseEditTemplate(string(data))

	if msg.isNew {
		// Validate and add new task
		if err := ValidateNewTask(m.state.Tasks, parsed.title, parsed.description, parsed.complexity, parsed.criteria, parsed.dependsOn); err != nil {
			m.confirmErr = fmt.Sprintf("Invalid task: %v", err)
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return clearConfirmErrMsg{}
			})
		}
		m.state.AddTask(parsed.title, parsed.description, parsed.complexity, parsed.criteria, parsed.dependsOn)
	} else {
		// Update existing task
		task := m.state.FindTask(msg.taskID)
		if task != nil {
			if parsed.title != "" {
				task.Title = parsed.title
			}
			if parsed.complexity != "" {
				task.Complexity = parsed.complexity
			}
			task.Description = parsed.description
			task.AcceptanceCriteria = parsed.criteria
			task.DependsOn = parsed.dependsOn
			task.PlanVersionModified = m.state.PlanVersion
		}
	}

	_ = state.Save(m.stateRoot, m.state)
	m.refreshList()
	return m, nil
}

// --- Helpers ---

func (m *ReviewModel) refreshList() {
	cursorID := m.taskList.CursorID()
	items := buildReviewItems(m.state)
	m.taskList.SetItems(items)
	if cursorID != "" {
		m.taskList.SetCursorByID(cursorID)
	}
}

func buildReviewItems(s *state.State) []components.TaskListItem {
	displayItems := BuildTaskDisplayList(s.Tasks)
	items := make([]components.TaskListItem, len(displayItems))
	for i, d := range displayItems {
		// Find the original task for detail formatting
		var detail string
		for _, t := range s.Tasks {
			if t.ID == d.ID {
				detail = FormatTaskDetail(t, s.Tasks)
				break
			}
		}

		items[i] = components.TaskListItem{
			ID:         d.ID,
			Title:      d.Title,
			Complexity: d.Complexity,
			Status:     components.TaskStatus(d.Status),
			Editable:   d.Editable,
			Detail:     detail,
		}
	}
	return items
}

func getEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	// Fallback chain
	for _, name := range []string{"nano", "vi", "notepad"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "nano"
}

// --- Edit Template Formatting/Parsing ---

func formatEditTemplate(task *state.Task) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Task: %s\n", task.ID)
	fmt.Fprintf(&b, "Status: %s (do not change)\n", task.Status)
	fmt.Fprintf(&b, "title: %s\n", task.Title)
	fmt.Fprintf(&b, "complexity: %s\n", task.Complexity)

	if len(task.DependsOn) > 0 {
		b.WriteString("depends_on:\n")
		for _, dep := range task.DependsOn {
			fmt.Fprintf(&b, "  - %s\n", dep)
		}
	} else {
		b.WriteString("depends_on:\n")
	}

	b.WriteString("\n## Description\n")
	b.WriteString(task.Description)
	b.WriteString("\n")

	b.WriteString("\n## Acceptance Criteria\n")
	for _, c := range task.AcceptanceCriteria {
		fmt.Fprintf(&b, "- %s\n", c)
	}

	return b.String()
}

func formatNewTemplate() string {
	var b strings.Builder

	b.WriteString("title: \n")
	b.WriteString("complexity: medium\n")
	b.WriteString("depends_on:\n")

	b.WriteString("\n## Description\n")
	b.WriteString("\n")

	b.WriteString("\n## Acceptance Criteria\n")
	b.WriteString("- \n")

	return b.String()
}

type parsedTemplate struct {
	title       string
	complexity  string
	dependsOn   []string
	description string
	criteria    []string
}

func parseEditTemplate(content string) parsedTemplate {
	var result parsedTemplate
	lines := strings.Split(content, "\n")

	section := "header" // header, description, criteria

	var descLines []string
	var criteriaLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Section headers
		if trimmed == "## Description" {
			section = "description"
			continue
		}
		if trimmed == "## Acceptance Criteria" {
			section = "criteria"
			continue
		}

		switch section {
		case "header":
			if strings.HasPrefix(trimmed, "title:") {
				result.title = strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
			} else if strings.HasPrefix(trimmed, "complexity:") {
				result.complexity = strings.TrimSpace(strings.TrimPrefix(trimmed, "complexity:"))
			} else if strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "- task") {
				// Skip non-task dependency lines
			} else if strings.HasPrefix(trimmed, "- task") || strings.HasPrefix(trimmed, "- task-") {
				dep := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				dep = strings.TrimSpace(dep)
				if dep != "" {
					result.dependsOn = append(result.dependsOn, dep)
				}
			}

		case "description":
			descLines = append(descLines, line)

		case "criteria":
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "• ") {
				criterion := strings.TrimPrefix(trimmed, "- ")
				criterion = strings.TrimPrefix(criterion, "• ")
				criterion = strings.TrimSpace(criterion)
				if criterion != "" {
					criteriaLines = append(criteriaLines, criterion)
				}
			}
		}
	}

	result.description = strings.TrimSpace(strings.Join(descLines, "\n"))
	result.criteria = criteriaLines

	return result
}
