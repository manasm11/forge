package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskStatus mirrors state.TaskStatus without importing state.
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in-progress"
	StatusDone       TaskStatus = "done"
	StatusFailed     TaskStatus = "failed"
	StatusSkipped    TaskStatus = "skipped"
	StatusCancelled  TaskStatus = "cancelled"
)

// TaskListItem represents a single item in the task list display.
type TaskListItem struct {
	ID         string
	Title      string
	Complexity string
	Status     TaskStatus
	Editable   bool
	Detail     string // pre-rendered detail text
}

// TaskSelectedMsg is emitted when the selected task changes.
type TaskSelectedMsg struct {
	Item TaskListItem
}

// TaskActionMsg is emitted when the user triggers an action on a task.
type TaskActionMsg struct {
	Action string // "edit", "delete", "new", "reorder_up", "reorder_down"
	TaskID string
}

// TaskListModel is a reusable list component for displaying tasks.
type TaskListModel struct {
	items      []TaskListItem
	cursor     int  // currently highlighted item
	scrollOff  int  // first visible item index
	detailView bool // whether to show expanded detail panel
	width      int
	height     int
}

// Styles for task list rendering.
var (
	selectedPrefix = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	doneIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Render("✅")

	failedIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Render("❌")

	progressIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")).
			Render("🔄")

	skippedIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render("⏭")

	complexityStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	detailBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280"))

	detailContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB")).
				PaddingLeft(1)
)

// NewTaskListModel creates a new task list component.
func NewTaskListModel(items []TaskListItem) TaskListModel {
	return TaskListModel{
		items:  items,
		cursor: 0,
	}
}

// SetItems replaces the items (e.g., after delete/reorder).
func (m *TaskListModel) SetItems(items []TaskListItem) {
	m.items = items
	if m.cursor >= len(items) {
		m.cursor = len(items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// SetSize updates the component dimensions.
func (m *TaskListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SelectedItem returns the currently highlighted item.
func (m TaskListModel) SelectedItem() *TaskListItem {
	if len(m.items) == 0 || m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	return &item
}

// CursorID returns the ID of the currently selected task.
func (m TaskListModel) CursorID() string {
	if item := m.SelectedItem(); item != nil {
		return item.ID
	}
	return ""
}

// SetCursorByID moves the cursor to the item with the given ID.
func (m *TaskListModel) SetCursorByID(id string) {
	for i, item := range m.items {
		if item.ID == id {
			m.cursor = i
			m.ensureVisible()
			return
		}
	}
}

// ToggleDetail toggles the expanded detail panel.
func (m *TaskListModel) ToggleDetail() {
	m.detailView = !m.detailView
}

// Init returns the initial command.
func (m TaskListModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the task list component.
func (m TaskListModel) Update(msg tea.Msg) (TaskListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.ensureVisible()
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
			return m, nil

		case "enter":
			m.detailView = !m.detailView
			return m, nil

		case "e":
			if item := m.SelectedItem(); item != nil && item.Editable {
				return m, func() tea.Msg {
					return TaskActionMsg{Action: "edit", TaskID: item.ID}
				}
			}
			return m, nil

		case "d":
			if item := m.SelectedItem(); item != nil && item.Editable {
				return m, func() tea.Msg {
					return TaskActionMsg{Action: "delete", TaskID: item.ID}
				}
			}
			return m, nil

		case "n":
			return m, func() tea.Msg {
				return TaskActionMsg{Action: "new"}
			}

		case "J": // shift+j = reorder down
			if item := m.SelectedItem(); item != nil && item.Editable {
				return m, func() tea.Msg {
					return TaskActionMsg{Action: "reorder_down", TaskID: item.ID}
				}
			}
			return m, nil

		case "K": // shift+k = reorder up
			if item := m.SelectedItem(); item != nil && item.Editable {
				return m, func() tea.Msg {
					return TaskActionMsg{Action: "reorder_up", TaskID: item.ID}
				}
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the task list component.
func (m TaskListModel) View() string {
	if m.width == 0 || m.height == 0 || len(m.items) == 0 {
		return ""
	}

	listHeight := m.height
	var detailHeight int
	if m.detailView {
		detailHeight = m.height * 40 / 100
		if detailHeight < 5 {
			detailHeight = 5
		}
		listHeight = m.height - detailHeight - 1 // -1 for separator
	}
	if listHeight < 1 {
		listHeight = 1
	}

	// Render list items
	var listLines []string
	visibleEnd := m.scrollOff + listHeight
	if visibleEnd > len(m.items) {
		visibleEnd = len(m.items)
	}

	for i := m.scrollOff; i < visibleEnd; i++ {
		listLines = append(listLines, m.renderItem(i))
	}

	listView := strings.Join(listLines, "\n")

	if !m.detailView {
		return listView
	}

	// Render detail panel
	separator := detailBorderStyle.Render(strings.Repeat("─", m.width))
	detailView := m.renderDetail(detailHeight)

	return lipgloss.JoinVertical(lipgloss.Left, listView, separator, detailView)
}

func (m *TaskListModel) ensureVisible() {
	listHeight := m.height
	if m.detailView {
		detailHeight := m.height * 40 / 100
		if detailHeight < 5 {
			detailHeight = 5
		}
		listHeight = m.height - detailHeight - 1
	}
	if listHeight < 1 {
		listHeight = 1
	}

	if m.cursor < m.scrollOff {
		m.scrollOff = m.cursor
	}
	if m.cursor >= m.scrollOff+listHeight {
		m.scrollOff = m.cursor - listHeight + 1
	}
}

func (m TaskListModel) renderItem(idx int) string {
	item := m.items[idx]
	isSelected := idx == m.cursor

	// Status icon
	var icon string
	switch item.Status {
	case StatusDone:
		icon = doneIcon
	case StatusFailed:
		icon = failedIcon
	case StatusInProgress:
		icon = progressIcon
	case StatusSkipped:
		icon = skippedIcon
	default:
		icon = "  " // blank for pending
	}

	// Complexity badge
	badge := complexityStyle.Render(fmt.Sprintf("[%s]", item.Complexity))

	// Build the line
	var prefix string
	style := normalStyle
	if isSelected {
		prefix = selectedPrefix.Render("→ ")
		style = selectedStyle
	} else {
		prefix = "  "
	}

	title := style.Render(item.Title)
	if !item.Editable && !isSelected {
		title = dimStyle.Render(item.Title)
	}

	line := fmt.Sprintf("%s%s %s %s %s", prefix, icon, item.ID, badge, title)

	// Truncate to width if needed
	if m.width > 0 && lipgloss.Width(line) > m.width {
		// Simple truncation — just limit the visible width
		line = line[:m.width-1] + "…"
	}

	return line
}

func (m TaskListModel) renderDetail(maxHeight int) string {
	item := m.SelectedItem()
	if item == nil || item.Detail == "" {
		return dimStyle.Render("  No task selected")
	}

	content := detailContentStyle.Render(item.Detail)

	// Truncate to max height
	lines := strings.Split(content, "\n")
	if len(lines) > maxHeight {
		lines = lines[:maxHeight]
	}

	return strings.Join(lines, "\n")
}
