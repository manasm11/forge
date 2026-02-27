package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/state"
)

type PlanningModel struct {
	width, height int
}

func NewPlanningModel() PlanningModel {
	return PlanningModel{}
}

func (m PlanningModel) Init() tea.Cmd {
	return nil
}

func (m PlanningModel) Update(msg tea.Msg) (PlanningModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseReview}
			}
		}
	}
	return m, nil
}

func (m PlanningModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1).
		Render("Phase 1: Interactive Planning")

	body := lipgloss.NewStyle().
		Foreground(Text).
		Render("This phase will let you chat with Claude to plan your project.")

	help := HelpStyle.Render("ctrl+n: continue to Issue Review →")

	content := lipgloss.JoinVertical(lipgloss.Left, title, body, "", help)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *PlanningModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
