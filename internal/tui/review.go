package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/state"
)

type ReviewModel struct {
	state         *state.State
	width, height int
}

func NewReviewModel(s *state.State) ReviewModel {
	return ReviewModel{state: s}
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

func (m ReviewModel) Update(msg tea.Msg) (ReviewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseInputs}
			}
		case "ctrl+p":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhasePlanning}
			}
		}
	}
	return m, nil
}

func (m ReviewModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1).
		Render("Phase 2: Task Review")

	taskCount := 0
	if m.state != nil {
		taskCount = len(m.state.Tasks)
	}

	body := lipgloss.NewStyle().
		Foreground(Text).
		Render(fmt.Sprintf("%d tasks generated from planning.\n[This will be a full interactive review in the next milestone]", taskCount))

	help := HelpStyle.Render("ctrl+n: continue to Input Collection →  |  ctrl+p: go back to Planning ←")

	content := lipgloss.JoinVertical(lipgloss.Left, title, body, "", help)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *ReviewModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
