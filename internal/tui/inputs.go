package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/state"
)

type InputsModel struct {
	width, height int
}

func NewInputsModel() InputsModel {
	return InputsModel{}
}

func (m InputsModel) Init() tea.Cmd {
	return nil
}

func (m InputsModel) Update(msg tea.Msg) (InputsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseExecution}
			}
		case "ctrl+p":
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseReview}
			}
		}
	}
	return m, nil
}

func (m InputsModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1).
		Render("Phase 3: Input Collection")

	body := lipgloss.NewStyle().
		Foreground(Text).
		Render("This phase will collect settings for autonomous execution.")

	help := HelpStyle.Render("ctrl+n: continue to Execution →  |  ctrl+p: go back to Issue Review ←")

	content := lipgloss.JoinVertical(lipgloss.Left, title, body, "", help)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *InputsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
