package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Primary   = lipgloss.Color("#7C3AED") // purple
	Secondary = lipgloss.Color("#06B6D4") // cyan
	Success   = lipgloss.Color("#10B981") // green
	Warning   = lipgloss.Color("#F59E0B") // amber
	Danger    = lipgloss.Color("#EF4444") // red
	Muted     = lipgloss.Color("#6B7280") // gray
	Text      = lipgloss.Color("#E5E7EB") // light gray

	// Reusable styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			PaddingLeft(1).
			PaddingRight(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Muted)

	StatusBar = lipgloss.NewStyle().
			Foreground(Text).
			Background(lipgloss.Color("#1F2937")).
			PaddingLeft(1).
			PaddingRight(1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(Muted).
			PaddingLeft(1)

	PhaseActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Secondary)

	PhaseLabelStyle = lipgloss.NewStyle().
			Foreground(Muted)
)
