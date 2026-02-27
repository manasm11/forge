package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ProgressBarModel is a simple reusable progress bar.
type ProgressBarModel struct {
	done  int
	total int
	width int
}

var (
	progressBarFilled = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981"))
	progressBarEmpty = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280"))
	progressBarText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))
)

// NewProgressBarModel creates a new progress bar.
func NewProgressBarModel(total, width int) ProgressBarModel {
	return ProgressBarModel{
		total: total,
		width: width,
	}
}

// SetDone updates the completed count.
func (m *ProgressBarModel) SetDone(done int) {
	m.done = done
}

// SetTotal updates the total count.
func (m *ProgressBarModel) SetTotal(total int) {
	m.total = total
}

// SetWidth updates the bar width.
func (m *ProgressBarModel) SetWidth(width int) {
	m.width = width
}

// View renders the progress bar.
func (m ProgressBarModel) View() string {
	barWidth := m.width - 20 // leave room for "  3/7 (43%)"
	if barWidth < 5 {
		barWidth = 5
	}
	if m.total == 0 {
		empty := strings.Repeat("░", barWidth)
		return fmt.Sprintf("  %s 0/0 (0%%)", progressBarEmpty.Render(empty))
	}

	pct := m.done * 100 / m.total
	filled := m.done * barWidth / m.total
	empty := barWidth - filled

	bar := progressBarFilled.Render(strings.Repeat("█", filled)) +
		progressBarEmpty.Render(strings.Repeat("░", empty))

	label := progressBarText.Render(fmt.Sprintf(" %d/%d (%d%%)", m.done, m.total, pct))

	return fmt.Sprintf("  %s%s", bar, label)
}
