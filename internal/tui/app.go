package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/executor"
	"github.com/manasm11/forge/internal/state"
)

// TransitionMsg signals a phase transition.
type TransitionMsg struct {
	To state.Phase
}

// AppModel is the root bubbletea model managing phase transitions.
type AppModel struct {
	state      *state.State
	stateRoot  string // project root directory
	claude     claude.Claude
	claudeExec executor.ClaudeExecutor
	program    *tea.Program
	phase      state.Phase
	planning   PlanningModel
	review     ReviewModel
	inputs     InputsModel
	execution  ExecutionModel
	width      int
	height     int
	err        error
	quitting   bool
}

// NewAppModel creates a new root model with the given state.
func NewAppModel(s *state.State, root string, claudeClient claude.Claude, claudeExec executor.ClaudeExecutor) AppModel {
	return AppModel{
		state:      s,
		stateRoot:  root,
		claude:     claudeClient,
		claudeExec: claudeExec,
		phase:      s.Phase,
		planning:   NewPlanningModel(s, root, claudeClient, nil),
		review:     NewReviewModel(s, root),
		inputs:     NewInputsModel(s, root),
	}
}

// SetProgram sets the tea.Program reference for streaming operations.
// Must be called after tea.NewProgram() and before p.Run().
func (m *AppModel) SetProgram(p *tea.Program) {
	m.program = p
	m.planning = NewPlanningModel(m.state, m.stateRoot, m.claude, p)
	m.execution.SetProgram(p)
}

func (m *AppModel) Init() tea.Cmd {
	switch m.phase {
	case state.PhaseInputs:
		return m.inputs.Init()
	case state.PhaseExecution:
		m.execution = NewExecutionModel(m.state, m.stateRoot, m.claudeExec)
		m.execution.SetProgram(m.program)
		return tea.Batch(m.execution.Init(), m.execution.StartExecution())
	default:
		return m.planning.Init()
	}
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for header and status bar
		contentHeight := m.height - 4
		if contentHeight < 0 {
			contentHeight = 0
		}

		m.planning.SetSize(m.width, contentHeight)
		m.review.SetSize(m.width, contentHeight)
		m.inputs.SetSize(m.width, contentHeight)
		m.execution.SetSize(m.width, contentHeight)

		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+p":
			// Go to previous phase
			return m, m.transitionToPrevPhase()
		case "ctrl+n":
			// Go to next phase
			return m, m.transitionToNextPhase()
		}

	case TransitionMsg:
		m.phase = msg.To
		m.state.Phase = msg.To
		if err := state.Save(m.stateRoot, m.state); err != nil {
			m.err = err
		}

		// Recreate phase models on transition
		var initCmd tea.Cmd
		switch msg.To {
		case state.PhasePlanning:
			m.planning = NewPlanningModel(m.state, m.stateRoot, m.claude, m.program)
		case state.PhaseReview:
			m.review = NewReviewModel(m.state, m.stateRoot)
		case state.PhaseInputs:
			m.inputs = NewInputsModel(m.state, m.stateRoot)
		case state.PhaseExecution:
			m.execution = NewExecutionModel(m.state, m.stateRoot, m.claudeExec)
			m.execution.SetProgram(m.program)
			m.execution.SetSize(m.width, m.height-4)
			initCmd = tea.Batch(m.execution.Init(), m.execution.StartExecution())
		}

		return m, initCmd
	}

	// Delegate to active phase
	var cmd tea.Cmd
	switch m.phase {
	case state.PhasePlanning:
		m.planning, cmd = m.planning.Update(msg)
	case state.PhaseReview:
		m.review, cmd = m.review.Update(msg)
	case state.PhaseInputs:
		m.inputs, cmd = m.inputs.Update(msg)
	case state.PhaseExecution:
		m.execution, cmd = m.execution.Update(msg)
	}

	return m, cmd
}

func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	// Header
	header := m.renderHeader()

	// Phase content
	var content string
	switch m.phase {
	case state.PhasePlanning:
		content = m.planning.View()
	case state.PhaseReview:
		content = m.review.View()
	case state.PhaseInputs:
		content = m.inputs.View()
	case state.PhaseExecution:
		content = m.execution.View()
	}

	// Error display
	if m.err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(Danger).
			Render(fmt.Sprintf("Error: %v", m.err))
		content = lipgloss.JoinVertical(lipgloss.Left, content, errMsg)
	}

	// Status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar)
}

func (m *AppModel) renderHeader() string {
	title := TitleStyle.Render("⚒ forge")

	phases := []struct {
		name  string
		phase state.Phase
	}{
		{"Planning", state.PhasePlanning},
		{"Review", state.PhaseReview},
		{"Inputs", state.PhaseInputs},
		{"Execution", state.PhaseExecution},
	}

	var phaseIndicators string
	for i, p := range phases {
		style := PhaseLabelStyle
		if p.phase == m.phase {
			style = PhaseActiveStyle
		}
		if i > 0 {
			phaseIndicators += SubtitleStyle.Render(" → ")
		}
		phaseIndicators += style.Render(p.name)
	}

	headerContent := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", phaseIndicators)

	headerBar := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("#1F2937")).
		PaddingLeft(1).
		Render(headerContent)

	return headerBar
}

// State returns the current state for external access (e.g., final save).
func (m *AppModel) State() *state.State {
	return m.state
}

func (m *AppModel) renderStatusBar() string {
	help := "ctrl+c: quit"
	if m.phase != state.PhasePlanning {
		help = "ctrl+p: prev  |  " + help
	}
	if m.phase != state.PhaseExecution {
		help = "ctrl+n: next  |  " + help
	}

	return StatusBar.
		Width(m.width).
		Render(help)
}

// transitionToPrevPhase moves to the previous phase in the workflow.
func (m *AppModel) transitionToPrevPhase() tea.Cmd {
	var prevPhase state.Phase
	switch m.phase {
	case state.PhaseReview:
		prevPhase = state.PhasePlanning
	case state.PhaseInputs:
		prevPhase = state.PhaseReview
	case state.PhaseExecution:
		prevPhase = state.PhaseInputs
	default:
		return nil
	}
	return func() tea.Msg {
		return TransitionMsg{To: prevPhase}
	}
}

// transitionToNextPhase moves to the next phase in the workflow.
// It validates that the transition is valid before proceeding.
func (m *AppModel) transitionToNextPhase() tea.Cmd {
	var nextPhase state.Phase
	var errMsg string

	switch m.phase {
	case state.PhasePlanning:
		// Check if there are any tasks defined
		if len(m.state.Tasks) == 0 {
			errMsg = "no tasks defined yet"
		} else {
			nextPhase = state.PhaseReview
		}
	case state.PhaseReview:
		// Use CanConfirm to validate task list
		errMsg = CanConfirm(m.state.Tasks)
		if errMsg == "" {
			nextPhase = state.PhaseInputs
		}
	case state.PhaseInputs:
		nextPhase = state.PhaseExecution
	default:
		return nil
	}

	if errMsg != "" {
		// Store error to display
		m.err = fmt.Errorf(errMsg)
		return nil
	}

	return func() tea.Msg {
		return TransitionMsg{To: nextPhase}
	}
}
