package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/state"
)

type ExecutionModel struct {
	width, height int
	showFinalMsg  bool
}

func NewExecutionModel() ExecutionModel {
	return ExecutionModel{}
}

func (m ExecutionModel) Init() tea.Cmd {
	return nil
}

func (m ExecutionModel) Update(msg tea.Msg) (ExecutionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n":
			m.showFinalMsg = true
			return m, nil
		case "ctrl+p":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseInputs}
			}
		}
	}
	return m, nil
}

func (m ExecutionModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1).
		Render("Phase 4: Execution")

	body := lipgloss.NewStyle().
		Foreground(Text).
		Render("This phase will autonomously implement issues using Claude Code.")

	var extra string
	if m.showFinalMsg {
		extra = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true).
			Render("This is the final phase.")
	}

	help := HelpStyle.Render("ctrl+p: go back to Input Collection ←")

	parts := []string{title, body}
	if extra != "" {
		parts = append(parts, "", extra)
	}
	parts = append(parts, "", help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *ExecutionModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
