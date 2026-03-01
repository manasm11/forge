package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/executor"
	"github.com/manasm11/forge/internal/state"
	"github.com/manasm11/forge/internal/tui/components"
)

// ExecutionEventMsg wraps executor.TaskEvent for the bubbletea message loop.
type ExecutionEventMsg struct {
	Event executor.TaskEvent
}

// ExecutionDoneMsg signals the runner has finished.
type ExecutionDoneMsg struct {
	Err error
}

// TickMsg is the 1-second heartbeat for updating elapsed times.
type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// ExecutionModel is the TUI model for the execution dashboard.
type ExecutionModel struct {
	state       *state.State
	stateRoot   string
	claude      executor.ClaudeExecutor
	program     *tea.Program
	progress    []TaskProgress
	logStream   components.LogStreamModel
	progressBar components.ProgressBarModel
	cursor      int // selected task in list
	status      ExecutionStatus
	summary     *ExecutionSummary
	width       int
	height      int
	startedAt   time.Time

	// Execution control
	cancelFunc context.CancelFunc
	started    bool // whether execution has been started
	userMoved  bool // user manually navigated away from running task
}

// NewExecutionModel creates a new execution dashboard.
func NewExecutionModel(s *state.State, root string, claude executor.ClaudeExecutor) ExecutionModel {
	settings := s.Settings
	if settings == nil {
		settings = &state.Settings{MaxRetries: 2}
	}

	progress := BuildTaskProgressList(s.Tasks, settings)

	// Count non-cancelled tasks for progress bar
	total := len(progress)

	// Count already-done tasks
	done := 0
	for _, tp := range progress {
		if tp.Status == state.TaskDone {
			done++
		}
	}

	m := ExecutionModel{
		state:       s,
		stateRoot:   root,
		claude:      claude,
		progress:    progress,
		logStream:   components.NewLogStreamModel(),
		progressBar: components.NewProgressBarModel(total, 30),
		status:      ExecRunning,
		startedAt:   time.Now(),
	}
	m.progressBar.SetDone(done)

	// Select first non-done task as cursor
	for i, tp := range progress {
		if tp.Status != state.TaskDone {
			m.cursor = i
			break
		}
	}

	return m
}

// SetProgram sets the tea.Program reference for receiving events from the runner goroutine.
func (m *ExecutionModel) SetProgram(p *tea.Program) {
	m.program = p
}

// Init starts the execution and the tick timer.
func (m ExecutionModel) Init() tea.Cmd {
	return tickCmd()
}

// StartExecution begins the runner in a background goroutine.
// Must be called after SetProgram.
func (m *ExecutionModel) StartExecution() tea.Cmd {
	if m.started || m.program == nil {
		return nil
	}
	m.started = true

	p := m.program
	s := m.state
	root := m.stateRoot
	claude := m.claude

	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		// Send cancel func back via a message so the model can store it
		p.Send(executionCancelFuncMsg{cancel: cancel})

		// Read context file
		contextContent := ""
		data, err := os.ReadFile(filepath.Join(root, ".forge", "context.md"))
		if err == nil {
			contextContent = string(data)
		}

		runner := executor.NewRunner(executor.RunnerConfig{
			State:       s,
			StateRoot:   root,
			Git:         executor.NewRealGitOps(root),
			Tests:       executor.NewRealTestRunner(root),
			Claude:      claude,
			ContextFile: contextContent,
			BaseBranch:  s.Settings.BaseBranch,
			RemoteURL:   s.Settings.RemoteURL,
			OnEvent: func(e executor.TaskEvent) {
				p.Send(ExecutionEventMsg{Event: e})
			},
		})

		runErr := runner.Run(ctx)
		return ExecutionDoneMsg{Err: runErr}
	}
}

// executionCancelFuncMsg carries the cancel function from the runner goroutine.
type executionCancelFuncMsg struct {
	cancel context.CancelFunc
}

// Update handles messages for the execution dashboard.
func (m ExecutionModel) Update(msg tea.Msg) (ExecutionModel, tea.Cmd) {
	switch msg := msg.(type) {

	case executionCancelFuncMsg:
		m.cancelFunc = msg.cancel
		return m, nil

	case ExecutionEventMsg:
		ApplyEventToProgress(m.progress, msg.Event)

		// Update log stream for current task
		if line := EventToLogLine(msg.Event); line != nil {
			for i := range m.progress {
				if m.progress[i].TaskID == msg.Event.TaskID {
					if m.cursor == i {
						m.logStream.AppendLine(components.LogLine{
							Text: line.Text,
							Type: components.LogLineType(line.Type),
						})
					}
					break
				}
			}
		}

		// Update progress bar
		done := 0
		for _, tp := range m.progress {
			if tp.Status == state.TaskDone {
				done++
			}
		}
		m.progressBar.SetDone(done)

		// Auto-advance cursor to running task (unless user manually navigated)
		if !m.userMoved {
			for i, tp := range m.progress {
				if tp.Status == state.TaskInProgress {
					if m.cursor != i {
						m.cursor = i
						m.logStream.SetLines(toComponentLogLines(m.progress[i].LogLines))
					}
					break
				}
			}
		}

		return m, nil

	case ExecutionDoneMsg:
		m.status = ComputeExecutionStatus(m.state.Tasks)
		s := ComputeExecutionSummary(m.progress)
		m.summary = &s
		return m, nil

	case TickMsg:
		if m.status != ExecRunning {
			return m, nil // stop ticking
		}
		// Update elapsed for in-progress tasks
		now := time.Now()
		for i := range m.progress {
			if m.progress[i].Status == state.TaskInProgress && m.progress[i].StartedAt != nil {
				m.progress[i].Elapsed = now.Sub(*m.progress[i].StartedAt)
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m ExecutionModel) handleKey(msg tea.KeyMsg) (ExecutionModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.progress)-1 {
			m.cursor++
			m.userMoved = true
			m.logStream.SetLines(toComponentLogLines(m.progress[m.cursor].LogLines))
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.userMoved = true
			m.logStream.SetLines(toComponentLogLines(m.progress[m.cursor].LogLines))
		}

	case "f": // follow running task again
		m.userMoved = false
		for i, tp := range m.progress {
			if tp.Status == state.TaskInProgress {
				m.cursor = i
				m.logStream.SetLines(toComponentLogLines(m.progress[i].LogLines))
				break
			}
		}

	case "l":
		// Open full log in $EDITOR
		if m.cursor >= 0 && m.cursor < len(m.progress) {
			taskID := m.progress[m.cursor].TaskID
			logPath := filepath.Join(m.stateRoot, ".forge", "logs", taskID+".log")
			if _, err := os.Stat(logPath); err == nil {
				editor := getEditor()
				c := exec.Command(editor, logPath)
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return nil
				})
			}
		}

	case "r":
		// Return to planning for replan (only when done or stopped)
		if m.status == ExecStopped || m.status == ExecComplete {
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhasePlanning}
			}
		}

	case "q":
		if m.status == ExecRunning {
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			m.status = ExecCancelled
			s := ComputeExecutionSummary(m.progress)
			m.summary = &s
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+p":
		// Go back to inputs (only when not running)
		if m.status != ExecRunning {
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseInputs}
			}
		}
	}

	return m, nil
}

// View renders the execution dashboard.
func (m ExecutionModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sections []string

	// Header line
	sections = append(sections, m.renderExecHeader())

	// Separator
	sections = append(sections, m.renderSeparator())

	// Task list
	taskListHeight := m.taskListHeight()
	sections = append(sections, m.renderTaskList(taskListHeight))

	// Separator
	sections = append(sections, m.renderSeparator())

	if m.summary != nil {
		// Show summary when done
		sections = append(sections, m.renderSummary())
	} else {
		// Log stream (selected task detail header + log)
		sections = append(sections, m.renderTaskDetailHeader())
		logHeight := m.logStreamHeight()
		m.logStream.SetSize(m.width, logHeight)
		sections = append(sections, m.logStream.View())
	}

	// Separator
	sections = append(sections, m.renderSeparator())

	// Progress bar
	m.progressBar.SetWidth(m.width - 4)
	sections = append(sections, m.progressBar.View())

	// Footer
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// SetSize updates the component dimensions.
func (m *ExecutionModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// --- Rendering helpers ---

func (m ExecutionModel) renderExecHeader() string {
	done := 0
	for _, tp := range m.progress {
		if tp.Status == state.TaskDone {
			done++
		}
	}
	total := len(m.progress)

	var statusText string
	switch m.status {
	case ExecComplete:
		statusText = "Execution Complete!"
	case ExecStopped:
		statusText = "Execution Stopped"
	case ExecCancelled:
		statusText = "Execution Cancelled"
	case ExecPaused:
		statusText = "Execution Paused"
	default:
		statusText = "Executing..."
	}

	left := lipgloss.NewStyle().
		Bold(true).
		Foreground(Secondary).
		Render(statusText)

	right := lipgloss.NewStyle().
		Foreground(Text).
		Render(fmt.Sprintf("Plan v%d · %d/%d tasks done", m.state.PlanVersion, done, total))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	return fmt.Sprintf(" %s%s%s", left, strings.Repeat(" ", gap), right)
}

func (m ExecutionModel) renderSeparator() string {
	return lipgloss.NewStyle().
		Foreground(Muted).
		Render("  " + strings.Repeat("─", m.width-4))
}

func (m ExecutionModel) renderTaskList(height int) string {
	if len(m.progress) == 0 {
		return lipgloss.NewStyle().Foreground(Muted).Render("  No tasks to execute")
	}

	var lines []string
	// Simple scroll: show tasks around cursor
	start := 0
	if len(m.progress) > height && m.cursor >= height {
		start = m.cursor - height + 1
	}
	end := start + height
	if end > len(m.progress) {
		end = len(m.progress)
	}

	for i := start; i < end; i++ {
		selected := i == m.cursor
		line := FormatTaskStatusLine(m.progress[i], selected, m.width-2)
		lines = append(lines, line)
	}

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m ExecutionModel) renderTaskDetailHeader() string {
	if m.cursor < 0 || m.cursor >= len(m.progress) {
		return ""
	}
	tp := m.progress[m.cursor]

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Text).
		Render(fmt.Sprintf("  %s: %s", tp.TaskID, tp.Title))

	var extra string
	if tp.Status == state.TaskInProgress && tp.Attempt > 0 {
		extra = lipgloss.NewStyle().
			Foreground(Warning).
			Render(fmt.Sprintf("  Attempt %d/%d", tp.Attempt, tp.MaxAttempts))
	}

	if extra != "" {
		return title + extra
	}
	return title
}

func (m ExecutionModel) renderSummary() string {
	if m.summary == nil {
		return ""
	}

	text := FormatSummaryText(*m.summary)
	lines := strings.Split(text, "\n")
	var styled []string
	for _, line := range lines {
		styled = append(styled, "  "+line)
	}
	return lipgloss.NewStyle().
		Foreground(Text).
		Render(strings.Join(styled, "\n"))
}

func (m ExecutionModel) renderFooter() string {
	var help string
	if m.status == ExecRunning {
		help = "  j/k navigate · f follow · l logs · q cancel"
	} else if m.status == ExecComplete {
		help = "  j/k navigate · l logs · r replan · ctrl+p back · q quit"
	} else if m.status == ExecStopped {
		help = "  j/k navigate · l logs · enter retry · r replan · ctrl+p back · q quit"
	} else {
		help = "  j/k navigate · l logs · r replan · ctrl+p back · q quit"
	}

	return HelpStyle.Render(help)
}

// --- Layout helpers ---

func (m ExecutionModel) taskListHeight() int {
	// Use ~30% of available height for task list, minimum 3
	h := m.height * 30 / 100
	if h < 3 {
		h = 3
	}
	// Cap at number of tasks
	if h > len(m.progress) {
		h = len(m.progress)
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m ExecutionModel) logStreamHeight() int {
	// Header(1) + sep(1) + taskList + sep(1) + detailHeader(1) + sep(1) + progressBar(1) + footer(1) = 7 + taskList
	overhead := 7 + m.taskListHeight()
	h := m.height - overhead
	if h < 3 {
		h = 3
	}
	return h
}

// toComponentLogLines converts tui.LogLine to components.LogLine.
func toComponentLogLines(lines []LogLine) []components.LogLine {
	result := make([]components.LogLine, len(lines))
	for i, l := range lines {
		result[i] = components.LogLine{
			Text: l.Text,
			Type: components.LogLineType(l.Type),
		}
	}
	return result
}

