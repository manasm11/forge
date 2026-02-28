package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/state"
	"github.com/manasm11/forge/internal/tui/components"
)

// PlanningModel manages the planning phase conversation with Claude.
type PlanningModel struct {
	chat             components.ChatModel
	state            *state.State
	stateRoot        string
	claude           claude.Claude // interface, not concrete type
	program          *tea.Program
	isReplanning     bool
	firstMessageSent bool
	restartConfirmed bool
	width, height    int
}

// restartMsg signals that the chat should be restarted.
type restartMsg struct{}

// NewPlanningModel creates a new planning phase model.
func NewPlanningModel(s *state.State, root string, claudeClient claude.Claude, p *tea.Program) PlanningModel {
	isReplanning := s.PlanVersion > 0 || len(s.Tasks) > 0

	m := PlanningModel{
		state:        s,
		stateRoot:    root,
		claude:       claudeClient,
		program:      p,
		isReplanning: isReplanning,
	}

	sender := m.createSender()
	handler := m.createSlashHandler()
	chat := components.NewChatModel(sender, handler)

	if isReplanning {
		replanCtx := BuildReplanContext(s)
		chat.AddMessage(components.RoleSystem, BuildReplanSystemMessage(replanCtx))

		// Restore previous conversation history
		for _, msg := range s.ConversationHistory {
			switch msg.Role {
			case "user":
				chat.AddMessage(components.RoleUser, msg.Content)
			case "assistant":
				chat.AddMessage(components.RoleAssistant, msg.Content)
			case "system":
				chat.AddMessage(components.RoleSystem, msg.Content)
			}
		}
	} else {
		welcome := "Welcome to Forge! \u2692\n\n" +
			"I'll help you plan your project through conversation.\n" +
			"Describe what you want to build and I'll ask questions to understand the details.\n\n" +
			"Commands: /done \u00b7 /summary \u00b7 /restart"
		chat.AddMessage(components.RoleSystem, welcome)

		// Show project snapshot if existing project detected
		if s.Snapshot != nil && s.Snapshot.IsExisting {
			snap := s.Snapshot
			var details strings.Builder
			details.WriteString("Detected existing project:\n")
			if snap.Language != "" {
				details.WriteString(fmt.Sprintf("  Language: %s\n", snap.Language))
			}
			details.WriteString(fmt.Sprintf("  Files: %d files (~%s lines)\n", snap.FileCount, formatLOC(snap.LOC)))
			if len(snap.Frameworks) > 0 {
				details.WriteString(fmt.Sprintf("  Frameworks: %s\n", strings.Join(snap.Frameworks, ", ")))
			}
			if snap.GitBranch != "" {
				commitInfo := ""
				if len(snap.RecentCommits) > 0 {
					commitInfo = fmt.Sprintf(", %d commits", len(snap.RecentCommits))
				}
				details.WriteString(fmt.Sprintf("  Git: %s branch%s\n", snap.GitBranch, commitInfo))
			}
			details.WriteString("\nI'll suggest changes that fit your existing codebase.")
			chat.AddMessage(components.RoleSystem, details.String())
		}
	}

	m.chat = chat
	return m
}

func (m PlanningModel) Init() tea.Cmd {
	return m.chat.Init()
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

	case components.StreamStartMsg:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		return m, cmd

	case components.StreamChunkMsg:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		return m, cmd

	case components.StreamDoneMsg:
		// Let chat handle UI cleanup
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		var cmds []tea.Cmd
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		if msg.Err != nil {
			return m, tea.Batch(cmds...)
		}

		// Check for final plan tags (initial planning)
		plan, err := claude.ExtractFinalPlan(msg.FullText)
		if err != nil {
			m.chat.AddMessage(components.RoleSystem, fmt.Sprintf("Error parsing plan: %v", err))
			return m, tea.Batch(cmds...)
		}
		if plan != nil {
			if err := m.applyFinalPlan(plan); err != nil {
				m.chat.AddMessage(components.RoleSystem, fmt.Sprintf("Error applying plan: %v", err))
				return m, tea.Batch(cmds...)
			}
			cmds = append(cmds, func() tea.Msg {
				return TransitionMsg{To: state.PhaseReview}
			})
			return m, tea.Batch(cmds...)
		}

		// Check for plan update tags (replanning)
		update, err := claude.ExtractPlanUpdate(msg.FullText)
		if err != nil {
			m.chat.AddMessage(components.RoleSystem, fmt.Sprintf("Error parsing plan update: %v", err))
			return m, tea.Batch(cmds...)
		}
		if update != nil {
			// Validate before applying
			warnings, valErr := ValidatePlanUpdate(m.state, update)
			if valErr != nil {
				m.chat.AddMessage(components.RoleSystem, fmt.Sprintf(
					"The plan update had an issue: %v\nCould you revise the update? Remember, completed tasks must stay as-is.", valErr))
				return m, tea.Batch(cmds...)
			}
			// Show warnings but proceed
			for _, w := range warnings {
				m.chat.AddMessage(components.RoleSystem, fmt.Sprintf("Note: %s", w))
			}
			if err := ApplyPlanUpdate(m.state, update); err != nil {
				m.chat.AddMessage(components.RoleSystem, fmt.Sprintf("Error applying plan update: %v", err))
				return m, tea.Batch(cmds...)
			}
			m.state.BumpPlanVersion(update.Summary)
			_ = state.Save(m.stateRoot, m.state)
			cmds = append(cmds, func() tea.Msg {
				return TransitionMsg{To: state.PhaseReview}
			})
			return m, tea.Batch(cmds...)
		}

		return m, tea.Batch(cmds...)

	case restartMsg:
		m.chat.ClearMessages()
		m.firstMessageSent = false
		m.restartConfirmed = false
		if m.isReplanning {
			replanCtx := BuildReplanContext(m.state)
			m.chat.AddMessage(components.RoleSystem, fmt.Sprintf(
				"Conversation restarted. You have %d completed, %d pending tasks.\nDescribe what changes you'd like.",
				replanCtx.CompletedCount, replanCtx.PendingCount))
		} else {
			m.state.ConversationHistory = nil
			m.chat.AddMessage(components.RoleSystem,
				"Chat restarted. Describe what you want to build!")
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.chat, cmd = m.chat.Update(msg)
	return m, cmd
}

func (m PlanningModel) View() string {
	return m.chat.View()
}

func (m *PlanningModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.chat.SetSize(w, h)
}

// SetProgram sets the tea.Program reference for streaming.
func (m *PlanningModel) SetProgram(p *tea.Program) {
	m.program = p
}

// createSender returns the MessageSender that communicates with Claude via streaming.
func (m *PlanningModel) createSender() components.MessageSender {
	return func(text string) tea.Cmd {
		return func() tea.Msg {
			// Save user message to conversation history
			m.state.AddConversationMessage("user", text)

			if m.claude == nil {
				return components.StreamDoneMsg{
					Err: fmt.Errorf("Claude CLI not available. Please install it and restart forge."),
				}
			}

			// Signal stream start
			if m.program != nil {
				m.program.Send(components.StreamStartMsg{})
			}

			var resp *claude.Response
			var err error

			onChunk := func(chunk string) {
				if m.program != nil {
					m.program.Send(components.StreamChunkMsg{Chunk: chunk})
				}
			}

			if !m.firstMessageSent {
				m.firstMessageSent = true
				prompt := m.buildFirstPrompt(text)
				resp, err = m.claude.SendStreaming(context.Background(), prompt, onChunk)
			} else {
				resp, err = m.claude.ContinueStreaming(context.Background(), text, onChunk)
			}

			// Save assistant response to conversation history
			if err == nil && resp != nil {
				m.state.AddConversationMessage("assistant", resp.Text)
				_ = state.Save(m.stateRoot, m.state)
			}

			fullText := ""
			if resp != nil {
				fullText = resp.Text
			}

			return components.StreamDoneMsg{
				FullText: fullText,
				Err:      err,
			}
		}
	}
}

// buildFirstPrompt constructs the initial prompt with system context.
func (m *PlanningModel) buildFirstPrompt(userMessage string) string {
	var prompt strings.Builder

	if m.isReplanning {
		replanCtx := BuildReplanContext(m.state)
		prompt.WriteString(BuildReplanPrompt(replanCtx))
	} else {
		prompt.WriteString(claude.InitialPlanningPrompt)

		// Append project context if available
		if m.state.Snapshot != nil && m.state.Snapshot.IsExisting {
			snap := m.state.Snapshot
			prompt.WriteString("\n\nEXISTING PROJECT CONTEXT:\n")
			if snap.Language != "" {
				fmt.Fprintf(&prompt, "Language: %s\n", snap.Language)
			}
			if len(snap.Frameworks) > 0 {
				fmt.Fprintf(&prompt, "Frameworks: %s\n", strings.Join(snap.Frameworks, ", "))
			}
			if len(snap.Dependencies) > 0 {
				fmt.Fprintf(&prompt, "Dependencies: %s\n", strings.Join(snap.Dependencies, ", "))
			}
			if snap.Structure != "" {
				fmt.Fprintf(&prompt, "Project Structure:\n%s\n", snap.Structure)
			}
			if len(snap.KeyFiles) > 0 {
				fmt.Fprintf(&prompt, "Key Files: %s\n", strings.Join(snap.KeyFiles, ", "))
			}
			if len(snap.RecentCommits) > 0 {
				prompt.WriteString("Recent Git History:\n")
				for _, c := range snap.RecentCommits {
					fmt.Fprintf(&prompt, "  %s\n", c)
				}
			}
			if snap.ReadmeContent != "" {
				fmt.Fprintf(&prompt, "README Summary:\n%s\n", snap.ReadmeContent)
			}
			if snap.ClaudeMD != "" {
				fmt.Fprintf(&prompt, "CLAUDE.md:\n%s\n", snap.ClaudeMD)
			}
		}
	}

	fmt.Fprintf(&prompt, "\n\nUser: %s", userMessage)
	return prompt.String()
}

// createSlashHandler returns the slash command handler for the planning phase.
func (m *PlanningModel) createSlashHandler() components.SlashHandler {
	return func(cmd components.SlashCommand) (tea.Cmd, bool) {
		switch cmd.Name {
		case "done":
			return m.handleSlashCommand("/done", m.doneInstruction()), true
		case "summary":
			return m.handleSlashCommand("/summary", "Please summarize your current understanding of the project and what you'd include in the plan."), true
		case "restart":
			return m.handleRestart(), true
		default:
			return nil, false
		}
	}
}

func (m *PlanningModel) doneInstruction() string {
	if m.isReplanning {
		return "The user has requested the updated plan. Based on everything discussed, generate the plan update now. Output inside <plan_update> tags with the JSON format specified."
	}
	return "The user has requested the final plan. Based on everything discussed, generate the plan now. Output inside <final_plan> tags with the JSON format specified."
}

// handleSlashCommand sends a command through the streaming sender.
func (m *PlanningModel) handleSlashCommand(cmdName, instruction string) tea.Cmd {
	if m.claude == nil {
		return func() tea.Msg {
			return components.StreamDoneMsg{
				Err: fmt.Errorf("Claude CLI not available"),
			}
		}
	}

	return func() tea.Msg {
		m.state.AddConversationMessage("user", cmdName)

		// Signal stream start
		if m.program != nil {
			m.program.Send(components.StreamStartMsg{})
		}

		var resp *claude.Response
		var err error

		onChunk := func(chunk string) {
			if m.program != nil {
				m.program.Send(components.StreamChunkMsg{Chunk: chunk})
			}
		}

		if !m.firstMessageSent {
			m.firstMessageSent = true
			prompt := m.buildFirstPrompt(instruction)
			resp, err = m.claude.SendStreaming(context.Background(), prompt, onChunk)
		} else {
			resp, err = m.claude.ContinueStreaming(context.Background(), instruction, onChunk)
		}

		if err == nil && resp != nil {
			m.state.AddConversationMessage("assistant", resp.Text)
			_ = state.Save(m.stateRoot, m.state)
		}

		fullText := ""
		if resp != nil {
			fullText = resp.Text
		}

		return components.StreamDoneMsg{
			FullText: fullText,
			Err:      err,
		}
	}
}

func (m *PlanningModel) handleRestart() tea.Cmd {
	if m.isReplanning && !m.restartConfirmed {
		m.restartConfirmed = true
		m.chat.AddMessage(components.RoleSystem,
			"This will reset the conversation but keep your existing tasks. Type /restart again to confirm.")
		return nil
	}
	return func() tea.Msg { return restartMsg{} }
}

// applyFinalPlan converts a PlanJSON into state tasks using the exported function.
func (m *PlanningModel) applyFinalPlan(plan *claude.PlanJSON) error {
	if err := ApplyInitialPlan(m.state, plan); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}
	if err := state.Save(m.stateRoot, m.state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

// formatLOC formats a line count for display (e.g., 3200 -> "3,200").
func formatLOC(loc int) string {
	s := fmt.Sprintf("%d", loc)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(ch)
	}
	return result.String()
}
