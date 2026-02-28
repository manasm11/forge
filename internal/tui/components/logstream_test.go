package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewLogStreamModel(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()

	if len(m.lines) != 0 {
		t.Errorf("lines = %d, want 0", len(m.lines))
	}
	if !m.follow {
		t.Error("follow = false, want true")
	}
}

func TestAppendLine(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 10)

	m.AppendLine(LogLine{Text: "first", Type: LogInfo})
	m.AppendLine(LogLine{Text: "second", Type: LogSuccess})

	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
	if m.lines[0].Text != "first" {
		t.Errorf("lines[0].Text = %q, want %q", m.lines[0].Text, "first")
	}
}

func TestSetLines(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 10)
	m.follow = false

	lines := []LogLine{
		{Text: "a", Type: LogInfo},
		{Text: "b", Type: LogError},
	}
	m.SetLines(lines)

	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
	if !m.follow {
		t.Error("SetLines should re-enable follow")
	}
}

func TestClear(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 10)

	m.AppendLine(LogLine{Text: "something", Type: LogInfo})
	m.Clear()

	if len(m.lines) != 0 {
		t.Errorf("lines = %d, want 0 after Clear", len(m.lines))
	}
	if m.offset != 0 {
		t.Errorf("offset = %d, want 0 after Clear", m.offset)
	}
}

func TestView_Empty(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 10)

	view := m.View()
	if !strings.Contains(view, "Waiting for events...") {
		t.Errorf("empty view should contain 'Waiting for events...', got %q", view)
	}
}

func TestView_ZeroDimensions(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	// Don't set size — width/height stay 0

	view := m.View()
	if view != "" {
		t.Errorf("zero dimension view should be empty, got %q", view)
	}
}

func TestView_RendersLines(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 10)

	m.AppendLine(LogLine{Text: "info line", Type: LogInfo})
	m.AppendLine(LogLine{Text: "success line", Type: LogSuccess})
	m.AppendLine(LogLine{Text: "error line", Type: LogError})
	m.AppendLine(LogLine{Text: "warning line", Type: LogWarning})
	m.AppendLine(LogLine{Text: "chunk line", Type: LogClaudeChunk})

	view := m.View()
	if view == "" {
		t.Error("View should produce non-empty output")
	}
	// All line texts should appear in the rendered output
	for _, text := range []string{"info line", "success line", "error line", "warning line", "chunk line"} {
		if !strings.Contains(view, text) {
			t.Errorf("view missing %q", text)
		}
	}
}

func TestUpdate_GKey(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 5)

	// Add more lines than visible
	for i := 0; i < 20; i++ {
		m.AppendLine(LogLine{Text: "line", Type: LogInfo})
	}

	// Press 'g' to scroll to top
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	if m.follow {
		t.Error("'g' should disable follow")
	}
	if m.offset != 0 {
		t.Errorf("'g' should scroll to top, offset = %d", m.offset)
	}
}

func TestUpdate_ShiftGKey(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 5)

	for i := 0; i < 20; i++ {
		m.AppendLine(LogLine{Text: "line", Type: LogInfo})
	}

	// Disable follow and scroll to top
	m.follow = false
	m.offset = 0

	// Press 'G' to scroll to bottom
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if !m.follow {
		t.Error("'G' should re-enable follow")
	}
	expectedOffset := len(m.lines) - m.height
	if m.offset != expectedOffset {
		t.Errorf("offset = %d, want %d", m.offset, expectedOffset)
	}
}

func TestScrollToBottom(t *testing.T) {
	t.Parallel()
	m := NewLogStreamModel()
	m.SetSize(80, 5)

	for i := 0; i < 20; i++ {
		m.AppendLine(LogLine{Text: "line", Type: LogInfo})
	}

	// follow is true by default, so scrollToBottom is called on each append
	expectedOffset := 20 - 5
	if m.offset != expectedOffset {
		t.Errorf("offset = %d, want %d (lines - height)", m.offset, expectedOffset)
	}
}
