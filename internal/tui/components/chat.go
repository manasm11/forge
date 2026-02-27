package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// Role identifies who sent a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message is a single chat message.
type Message struct {
	Role    Role
	Content string
	Time    time.Time
}

// SlashCommand represents a parsed slash command.
type SlashCommand struct {
	Name string
	Args string
}

// ResponseMsg is produced by MessageSender when a response arrives.
type ResponseMsg struct {
	Content string
	Err     error
}

// StreamStartMsg indicates a streaming response has begun.
type StreamStartMsg struct{}

// StreamChunkMsg carries a chunk of text from the streaming response.
type StreamChunkMsg struct {
	Chunk string
}

// StreamDoneMsg indicates the stream is complete.
type StreamDoneMsg struct {
	FullText string
	Err      error
}

// MessageSender is called when the user sends a message.
// It receives the user's text and returns a tea.Cmd that will
// eventually produce a ResponseMsg.
type MessageSender func(text string) tea.Cmd

// SlashHandler is called when a slash command is detected.
// Returns a tea.Cmd if the command produces an async action, nil otherwise.
// Returns a bool indicating if the command was handled.
type SlashHandler func(cmd SlashCommand) (tea.Cmd, bool)

// ChatModel is a reusable chat UI component.
type ChatModel struct {
	messages        []Message
	viewport        viewport.Model
	textInput       textinput.Model
	spinner         spinner.Model
	sender          MessageSender
	slashHandler    SlashHandler
	waiting         bool
	streaming       bool // true while receiving stream chunks
	streamingMsgIdx int  // index of the message being streamed into
	width           int
	height          int
	ready           bool
}

// Styles for chat rendering.
var (
	userNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED"))

	assistantNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#06B6D4"))

	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			PaddingLeft(1)

	assistantMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB")).
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("#06B6D4")).
				PaddingLeft(1)

	systemMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true).
			PaddingLeft(2)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#6B7280")).
				Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4"))
)

const inputAreaHeight = 3 // border top + input line + border bottom
const msgPadding = 2      // horizontal padding for message content

// NewChatModel creates a new chat component.
func NewChatModel(sender MessageSender, slashHandler SlashHandler) ChatModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message... (/ for commands)"
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	vp := viewport.New(0, 0)

	return ChatModel{
		messages:     nil,
		viewport:     vp,
		textInput:    ti,
		spinner:      sp,
		sender:       sender,
		slashHandler: slashHandler,
	}
}

// Init returns the initial commands for the chat component.
func (m ChatModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// Update handles messages for the chat component.
func (m ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case ResponseMsg:
		m.waiting = false
		if msg.Err != nil {
			m.addMessage(RoleSystem, fmt.Sprintf("Error: %v", msg.Err))
		} else {
			m.addMessage(RoleAssistant, msg.Content)
		}
		m.refreshViewport()
		return m, nil

	case StreamStartMsg:
		m.streaming = true
		m.waiting = true
		m.addMessage(RoleAssistant, "")
		m.streamingMsgIdx = len(m.messages) - 1
		m.refreshViewport()
		return m, m.spinner.Tick

	case StreamChunkMsg:
		if m.streaming && m.streamingMsgIdx >= 0 && m.streamingMsgIdx < len(m.messages) {
			m.messages[m.streamingMsgIdx].Content += msg.Chunk
			atBottom := m.viewport.AtBottom()
			m.viewport.SetContent(m.renderMessages())
			if atBottom {
				m.viewport.GotoBottom()
			}
		}
		return m, nil

	case StreamDoneMsg:
		m.streaming = false
		m.waiting = false
		if msg.Err != nil {
			if m.streamingMsgIdx >= 0 && m.streamingMsgIdx < len(m.messages) {
				if m.messages[m.streamingMsgIdx].Content == "" {
					// Replace empty streaming message with error
					m.messages[m.streamingMsgIdx].Role = RoleSystem
					m.messages[m.streamingMsgIdx].Content = fmt.Sprintf("Error: %v", msg.Err)
				} else {
					// Append error as new system message
					m.addMessage(RoleSystem, fmt.Sprintf("Error: %v", msg.Err))
				}
			}
		}
		m.refreshViewport()
		m.textInput.Focus()
		return m, nil

	case spinner.TickMsg:
		if m.waiting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			// Re-render to show updated spinner frame
			m.refreshViewport()
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.String() == "enter" && !m.waiting {
			text := strings.TrimSpace(m.textInput.Value())
			if text == "" {
				return m, nil
			}

			m.textInput.SetValue("")

			// Check for slash command
			if cmd, ok := ParseSlashCommand(text); ok {
				if m.slashHandler != nil {
					asyncCmd, handled := m.slashHandler(cmd)
					if handled {
						if asyncCmd != nil {
							m.waiting = true
							cmds = append(cmds, asyncCmd, m.spinner.Tick)
						}
						return m, tea.Batch(cmds...)
					}
				}
				// Unhandled slash command
				m.addMessage(RoleSystem, fmt.Sprintf("Unknown command: /%s", cmd.Name))
				m.refreshViewport()
				return m, nil
			}

			// Regular message
			m.addMessage(RoleUser, text)
			m.waiting = true
			cmds = append(cmds, m.sender(text), m.spinner.Tick)
			m.refreshViewport()
			return m, tea.Batch(cmds...)
		}
	}

	// Update text input only when not waiting
	if !m.waiting {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Always allow viewport scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the chat component.
func (m ChatModel) View() string {
	if m.width == 0 {
		return ""
	}

	// Input area
	var inputView string
	if m.waiting {
		spinnerText := fmt.Sprintf("%s Claude is thinking...", m.spinner.View())
		inputView = inputBorderStyle.Width(m.width - 4).Render(spinnerText)
	} else {
		m.textInput.Width = m.width - 6 // account for border + padding
		inputView = inputBorderStyle.Width(m.width - 4).Render(m.textInput.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), inputView)
}

// AddMessage adds a message to the chat from outside.
func (m *ChatModel) AddMessage(role Role, content string) {
	m.addMessage(role, content)
	m.refreshViewport()
}

// Messages returns a copy of all messages.
func (m ChatModel) Messages() []Message {
	msgs := make([]Message, len(m.messages))
	copy(msgs, m.messages)
	return msgs
}

// IsWaiting returns true if waiting for a response.
func (m ChatModel) IsWaiting() bool {
	return m.waiting
}

// SetSize updates the component dimensions.
func (m *ChatModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	vpHeight := height - inputAreaHeight
	if vpHeight < 0 {
		vpHeight = 0
	}

	m.viewport.Width = width
	m.viewport.Height = vpHeight
	m.ready = true
	m.refreshViewport()
}

// ReceiveResponse adds an assistant response to the chat and clears the waiting state.
// Returns nil. This is used when the parent model intercepts response messages
// (e.g., to check for plan tags) before forwarding the text to the chat.
func (m *ChatModel) ReceiveResponse(content string) tea.Cmd {
	m.waiting = false
	m.addMessage(RoleAssistant, content)
	m.refreshViewport()
	return nil
}

// ClearMessages removes all messages.
func (m *ChatModel) ClearMessages() {
	m.messages = nil
	m.streaming = false
	m.refreshViewport()
}

func (m *ChatModel) addMessage(role Role, content string) {
	m.messages = append(m.messages, Message{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	})
}

func (m *ChatModel) refreshViewport() {
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
}

func (m ChatModel) renderMessages() string {
	if len(m.messages) == 0 && !m.waiting {
		return ""
	}

	contentWidth := m.width - msgPadding*2
	if contentWidth < 10 {
		contentWidth = 10
	}

	var sb strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			sb.WriteString("\n")
		}
		rendered := m.renderMessage(msg, contentWidth)
		// Add cursor indicator for currently-streaming message
		if m.streaming && i == m.streamingMsgIdx {
			rendered += "▊"
		}
		sb.WriteString(rendered)
	}

	if m.waiting && !m.streaming {
		if len(m.messages) > 0 {
			sb.WriteString("\n\n")
		}
		thinkingText := fmt.Sprintf("  %s Claude is thinking...", m.spinner.View())
		sb.WriteString(assistantNameStyle.Render(thinkingText))
	}

	return sb.String()
}

func (m ChatModel) renderMessage(msg Message, width int) string {
	timestamp := msg.Time.Format("3:04 PM")
	wrapped := wordwrap.String(msg.Content, width-4) // account for border + padding

	switch msg.Role {
	case RoleUser:
		header := fmt.Sprintf("%s %s",
			userNameStyle.Render("You"),
			timeStyle.Render(timestamp),
		)
		body := userMsgStyle.Width(width).Render(wrapped)
		return fmt.Sprintf("%s\n%s", header, body)

	case RoleAssistant:
		header := fmt.Sprintf("%s %s",
			assistantNameStyle.Render("Claude"),
			timeStyle.Render(timestamp),
		)
		body := assistantMsgStyle.Width(width).Render(wrapped)
		return fmt.Sprintf("%s\n%s", header, body)

	case RoleSystem:
		return systemMsgStyle.Render(fmt.Sprintf("i %s", wrapped))

	default:
		return wrapped
	}
}

// ParseSlashCommand parses a slash command from input.
// Exported for testing.
func ParseSlashCommand(input string) (SlashCommand, bool) {
	if !strings.HasPrefix(input, "/") {
		return SlashCommand{}, false
	}

	trimmed := strings.TrimPrefix(input, "/")
	if trimmed == "" {
		return SlashCommand{}, false
	}

	parts := strings.SplitN(trimmed, " ", 2)
	cmd := SlashCommand{Name: parts[0]}
	if len(parts) > 1 {
		cmd.Args = strings.TrimSpace(parts[1])
	}

	return cmd, true
}
