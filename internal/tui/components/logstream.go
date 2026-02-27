package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogLineType classifies log line severity/purpose.
type LogLineType int

const (
	LogInfo LogLineType = iota
	LogSuccess
	LogError
	LogWarning
	LogClaudeChunk
)

// LogLine is a single line in the task's live log.
type LogLine struct {
	Text string
	Type LogLineType
}

// LogStreamModel is a streaming log viewer that auto-scrolls and shows color-coded lines.
type LogStreamModel struct {
	lines    []LogLine
	offset   int // scroll offset (first visible line)
	width    int
	height   int
	follow   bool // auto-scroll to bottom
}

// Styles for log rendering.
var (
	logInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))
	logSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))
	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))
	logWarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))
	logChunkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

// NewLogStreamModel creates a new log stream viewer.
func NewLogStreamModel() LogStreamModel {
	return LogStreamModel{
		follow: true,
	}
}

// SetSize updates the dimensions.
func (m *LogStreamModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// AppendLine adds a new log line and auto-scrolls if following.
func (m *LogStreamModel) AppendLine(line LogLine) {
	m.lines = append(m.lines, line)
	if m.follow {
		m.scrollToBottom()
	}
}

// SetLines replaces all lines (e.g., when switching to a different task).
func (m *LogStreamModel) SetLines(lines []LogLine) {
	m.lines = lines
	m.follow = true
	m.scrollToBottom()
}

// Clear removes all lines.
func (m *LogStreamModel) Clear() {
	m.lines = nil
	m.offset = 0
}

func (m *LogStreamModel) scrollToBottom() {
	if len(m.lines) > m.height && m.height > 0 {
		m.offset = len(m.lines) - m.height
	} else {
		m.offset = 0
	}
}

// Update handles scroll keys.
func (m LogStreamModel) Update(msg tea.Msg) (LogStreamModel, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "G": // scroll to bottom, re-enable follow
			m.follow = true
			m.scrollToBottom()
		case "g": // scroll to top
			m.follow = false
			m.offset = 0
		}
	}
	return m, nil
}

// View renders the log stream.
func (m LogStreamModel) View() string {
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	if len(m.lines) == 0 {
		return logChunkStyle.Render("  Waiting for events...")
	}

	visibleEnd := m.offset + m.height
	if visibleEnd > len(m.lines) {
		visibleEnd = len(m.lines)
	}
	start := m.offset
	if start < 0 {
		start = 0
	}

	var rendered []string
	for i := start; i < visibleEnd; i++ {
		rendered = append(rendered, m.renderLine(m.lines[i]))
	}

	// Pad with empty lines if needed
	for len(rendered) < m.height {
		rendered = append(rendered, "")
	}

	return strings.Join(rendered, "\n")
}

func (m LogStreamModel) renderLine(line LogLine) string {
	prefix := "  > "
	var style lipgloss.Style

	switch line.Type {
	case LogSuccess:
		style = logSuccessStyle
	case LogError:
		style = logErrorStyle
	case LogWarning:
		style = logWarningStyle
	case LogClaudeChunk:
		style = logChunkStyle
		prefix = "    "
	default:
		style = logInfoStyle
	}

	text := line.Text
	// For multi-line text, only show first line in the stream view
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}

	maxTextWidth := m.width - len(prefix) - 1
	if maxTextWidth > 0 && len(text) > maxTextWidth {
		text = text[:maxTextWidth-1] + "…"
	}

	return style.Render(fmt.Sprintf("%s%s", prefix, text))
}
