package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleItems() []TaskListItem {
	return []TaskListItem{
		{ID: "task-001", Title: "First task", Complexity: "low", Status: StatusPending, Editable: true, Detail: "Detail 1"},
		{ID: "task-002", Title: "Second task", Complexity: "medium", Status: StatusPending, Editable: true, Detail: "Detail 2"},
		{ID: "task-003", Title: "Third task", Complexity: "high", Status: StatusDone, Editable: false, Detail: "Detail 3"},
	}
}

func TestNewTaskListModel(t *testing.T) {
	t.Parallel()
	items := sampleItems()
	m := NewTaskListModel(items)

	if len(m.items) != 3 {
		t.Errorf("items = %d, want 3", len(m.items))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestTaskList_NavigateDown(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("cursor after 2x j = %d, want 2", m.cursor)
	}

	// Should not go past last item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("cursor should stop at last item, got %d", m.cursor)
	}
}

func TestTaskList_NavigateUp(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	// Move to bottom first
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 1 {
		t.Errorf("cursor after k = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("cursor after 2x k = %d, want 0", m.cursor)
	}

	// Should not go below 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", m.cursor)
	}
}

func TestTaskList_EditAction(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatal("'e' on editable item should produce a command")
	}
	msg := cmd()
	action, ok := msg.(TaskActionMsg)
	if !ok {
		t.Fatalf("expected TaskActionMsg, got %T", msg)
	}
	if action.Action != "edit" {
		t.Errorf("action = %q, want %q", action.Action, "edit")
	}
	if action.TaskID != "task-001" {
		t.Errorf("taskID = %q, want %q", action.TaskID, "task-001")
	}
}

func TestTaskList_DeleteAction(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("'d' on editable item should produce a command")
	}
	msg := cmd()
	action, ok := msg.(TaskActionMsg)
	if !ok {
		t.Fatalf("expected TaskActionMsg, got %T", msg)
	}
	if action.Action != "delete" {
		t.Errorf("action = %q, want %q", action.Action, "delete")
	}
}

func TestTaskList_NewAction(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("'n' should produce a command")
	}
	msg := cmd()
	action, ok := msg.(TaskActionMsg)
	if !ok {
		t.Fatalf("expected TaskActionMsg, got %T", msg)
	}
	if action.Action != "new" {
		t.Errorf("action = %q, want %q", action.Action, "new")
	}
}

func TestTaskList_ReorderDownAction(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if cmd == nil {
		t.Fatal("'J' on editable item should produce a command")
	}
	msg := cmd()
	action, ok := msg.(TaskActionMsg)
	if !ok {
		t.Fatalf("expected TaskActionMsg, got %T", msg)
	}
	if action.Action != "reorder_down" {
		t.Errorf("action = %q, want %q", action.Action, "reorder_down")
	}
}

func TestTaskList_ReorderUpAction(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if cmd == nil {
		t.Fatal("'K' on editable item should produce a command")
	}
	msg := cmd()
	action, ok := msg.(TaskActionMsg)
	if !ok {
		t.Fatalf("expected TaskActionMsg, got %T", msg)
	}
	if action.Action != "reorder_up" {
		t.Errorf("action = %q, want %q", action.Action, "reorder_up")
	}
}

func TestTaskList_EditOnNonEditable(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	// Move to task-003 (non-editable done task)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd != nil {
		t.Error("'e' on non-editable item should not produce a command")
	}
}

func TestTaskList_SelectedItem(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())

	item := m.SelectedItem()
	if item == nil {
		t.Fatal("SelectedItem should not be nil")
	}
	if item.ID != "task-001" {
		t.Errorf("SelectedItem().ID = %q, want %q", item.ID, "task-001")
	}
}

func TestTaskList_SelectedItem_Empty(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(nil)

	if m.SelectedItem() != nil {
		t.Error("SelectedItem on empty list should be nil")
	}
}

func TestTaskList_CursorID(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())

	if id := m.CursorID(); id != "task-001" {
		t.Errorf("CursorID = %q, want %q", id, "task-001")
	}
}

func TestTaskList_CursorID_Empty(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(nil)

	if id := m.CursorID(); id != "" {
		t.Errorf("CursorID on empty list = %q, want empty", id)
	}
}

func TestTaskList_SetItems(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	// Move cursor to last item
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Replace with fewer items — cursor should clamp
	newItems := []TaskListItem{
		{ID: "task-010", Title: "Only task", Editable: true},
	}
	m.SetItems(newItems)

	if len(m.items) != 1 {
		t.Errorf("items = %d, want 1", len(m.items))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestTaskList_SetItems_EmptyList(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())

	m.SetItems(nil)

	if m.cursor != 0 {
		t.Errorf("cursor should be 0 for empty list, got %d", m.cursor)
	}
}

func TestTaskList_SetCursorByID(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	m.SetCursorByID("task-003")
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}
}

func TestTaskList_SetCursorByID_NotFound(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())

	m.SetCursorByID("nonexistent")
	// Cursor should remain at 0 (unchanged)
	if m.cursor != 0 {
		t.Errorf("cursor should not change for nonexistent ID, got %d", m.cursor)
	}
}

func TestTaskList_ToggleDetail(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())

	if m.detailView {
		t.Error("detailView should start false")
	}

	m.ToggleDetail()
	if !m.detailView {
		t.Error("detailView should be true after toggle")
	}

	m.ToggleDetail()
	if m.detailView {
		t.Error("detailView should be false after second toggle")
	}
}

func TestTaskList_EnterTogglesDetail(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.detailView {
		t.Error("Enter should toggle detailView to true")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.detailView {
		t.Error("Enter again should toggle detailView back to false")
	}
}

func TestTaskList_View_Empty(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(nil)
	m.SetSize(80, 24)

	view := m.View()
	if view != "" {
		t.Errorf("View of empty list should be empty, got %q", view)
	}
}

func TestTaskList_View_NonEmpty(t *testing.T) {
	t.Parallel()
	m := NewTaskListModel(sampleItems())
	m.SetSize(80, 24)

	view := m.View()
	if view == "" {
		t.Error("View should produce non-empty output for non-empty list")
	}
}
