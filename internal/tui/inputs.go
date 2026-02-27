package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/generator"
	"github.com/manasm11/forge/internal/state"
)

// editorDoneMsg is sent when $EDITOR closes for the extra context field.
type editorDoneMsg struct {
	err     error
	tmpPath string
}

// clearFlashMsg clears the flash message after a timeout.
type clearFlashMsg struct{}

// InputsModel manages the input collection phase.
type InputsModel struct {
	fields     []InputField
	mcpServers []MCPServer
	maxTurns   MaxTurnsConfig
	cursor     int // index into the combined navigation items
	textInputs []textinput.Model
	state      *state.State
	stateRoot  string
	width      int
	height     int
	flashMsg   string
	flashErr   bool // true if flashMsg is an error
}

// Navigation sections: fields (0..len(fields)-1), then MCP servers, then max turns fields.
// We track which "zone" the cursor is in.

const (
	zoneFields     = 0
	zoneMCP        = 1
	zoneMaxTurns   = 2
)

func NewInputsModel(s *state.State, root string) InputsModel {
	fields := DefaultInputFields(s.Snapshot)
	mcpServers := DefaultMCPServers()
	maxTurns := DefaultMaxTurns()

	// Pre-populate from existing settings if available
	if s.Settings != nil {
		populateFromSettings(fields, s.Settings)
		populateMCPFromSettings(mcpServers, s.Settings)
		if s.Settings.MaxTurns.Small > 0 {
			maxTurns.Small = s.Settings.MaxTurns.Small
		}
		if s.Settings.MaxTurns.Medium > 0 {
			maxTurns.Medium = s.Settings.MaxTurns.Medium
		}
		if s.Settings.MaxTurns.Large > 0 {
			maxTurns.Large = s.Settings.MaxTurns.Large
		}
	}

	// Create text inputs for text/number fields
	var textInputs []textinput.Model
	for _, f := range fields {
		ti := textinput.New()
		ti.CharLimit = 256
		switch f.FieldType {
		case FieldText, FieldNumber:
			if f.Value != "" {
				ti.SetValue(f.Value)
			} else if f.Default != "" {
				ti.SetValue(f.Default)
			}
			ti.Placeholder = f.Default
		default:
			// Toggle/Editor fields don't use text inputs but we keep alignment
			if f.Value != "" {
				ti.SetValue(f.Value)
			} else if f.Default != "" {
				ti.SetValue(f.Default)
			}
		}
		textInputs = append(textInputs, ti)
	}

	// Focus first text input
	if len(textInputs) > 0 {
		textInputs[0].Focus()
	}

	m := InputsModel{
		fields:     fields,
		mcpServers: mcpServers,
		maxTurns:   maxTurns,
		cursor:     0,
		textInputs: textInputs,
		state:      s,
		stateRoot:  root,
	}

	return m
}

func populateFromSettings(fields []InputField, settings *state.Settings) {
	for i := range fields {
		switch fields[i].Key {
		case "test_command":
			if settings.TestCommand != "" {
				fields[i].Value = settings.TestCommand
			}
		case "build_command":
			if settings.BuildCommand != "" {
				fields[i].Value = settings.BuildCommand
			}
		case "branch_pattern":
			if settings.BranchPattern != "" {
				fields[i].Value = settings.BranchPattern
			}
		case "max_retries":
			fields[i].Value = fmt.Sprintf("%d", settings.MaxRetries)
		case "auto_pr":
			if settings.AutoPR {
				fields[i].Value = "true"
			} else {
				fields[i].Value = "false"
			}
		case "claude_model":
			if settings.ClaudeModel != "" {
				fields[i].Value = settings.ClaudeModel
			}
		case "extra_context":
			if settings.ExtraContext != "" {
				fields[i].Value = settings.ExtraContext
			}
		}
	}
}

func populateMCPFromSettings(servers []MCPServer, settings *state.Settings) {
	configMap := make(map[string]bool)
	for _, c := range settings.MCPServers {
		configMap[c.Name] = true
	}
	for i := range servers {
		if _, ok := configMap[servers[i].Name]; ok {
			servers[i].Enabled = true
		}
	}
}

func (m InputsModel) Init() tea.Cmd {
	return textinput.Blink
}

// totalItems returns the total number of navigable items.
func (m InputsModel) totalItems() int {
	return len(m.fields) + len(m.mcpServers)
}

// cursorZone returns which zone the cursor is in and the local index.
func (m InputsModel) cursorZone() (zone int, localIdx int) {
	if m.cursor < len(m.fields) {
		return zoneFields, m.cursor
	}
	return zoneMCP, m.cursor - len(m.fields)
}

func (m InputsModel) Update(msg tea.Msg) (InputsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			return m.moveCursor(1), nil
		case "shift+tab", "up":
			return m.moveCursor(-1), nil
		case "c":
			// Don't capture 'c' if typing in a text input
			zone, _ := m.cursorZone()
			if zone == zoneFields {
				f := m.fields[m.cursor]
				if f.FieldType == FieldText || f.FieldType == FieldNumber {
					break // let text input handle it
				}
			}
			return m.confirm()
		case "b":
			zone, _ := m.cursorZone()
			if zone == zoneFields {
				f := m.fields[m.cursor]
				if f.FieldType == FieldText || f.FieldType == FieldNumber {
					break // let text input handle it
				}
			}
			return m, func() tea.Msg {
				return TransitionMsg{To: state.PhaseReview}
			}
		case "q":
			zone, _ := m.cursorZone()
			if zone == zoneFields {
				f := m.fields[m.cursor]
				if f.FieldType == FieldText || f.FieldType == FieldNumber {
					break // let text input handle it
				}
			}
			return m, tea.Quit
		case " ":
			return m.handleSpace()
		case "enter":
			return m.handleEnter()
		}

	case editorDoneMsg:
		defer os.Remove(msg.tmpPath)
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Editor error: %v", msg.err)
			m.flashErr = true
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return clearFlashMsg{}
			})
		}
		data, err := os.ReadFile(msg.tmpPath)
		if err != nil {
			m.flashMsg = fmt.Sprintf("Could not read temp file: %v", err)
			m.flashErr = true
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return clearFlashMsg{}
			})
		}
		// Find the extra_context field and set its value
		for i := range m.fields {
			if m.fields[i].Key == "extra_context" {
				m.fields[i].Value = strings.TrimSpace(string(data))
				m.textInputs[i].SetValue(m.fields[i].Value)
				break
			}
		}
		return m, nil

	case clearFlashMsg:
		m.flashMsg = ""
		m.flashErr = false
		return m, nil
	}

	// Delegate to active text input
	zone, localIdx := m.cursorZone()
	if zone == zoneFields && localIdx < len(m.textInputs) {
		f := m.fields[localIdx]
		if f.FieldType == FieldText || f.FieldType == FieldNumber {
			var cmd tea.Cmd
			m.textInputs[localIdx], cmd = m.textInputs[localIdx].Update(msg)
			m.fields[localIdx].Value = m.textInputs[localIdx].Value()
			return m, cmd
		}
	}

	return m, nil
}

func (m InputsModel) moveCursor(delta int) InputsModel {
	total := m.totalItems()
	if total == 0 {
		return m
	}

	// Blur current text input
	zone, localIdx := m.cursorZone()
	if zone == zoneFields && localIdx < len(m.textInputs) {
		m.textInputs[localIdx].Blur()
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= total {
		m.cursor = total - 1
	}

	// Focus new text input
	zone, localIdx = m.cursorZone()
	if zone == zoneFields && localIdx < len(m.textInputs) {
		f := m.fields[localIdx]
		if f.FieldType == FieldText || f.FieldType == FieldNumber {
			m.textInputs[localIdx].Focus()
		}
	}

	return m
}

func (m InputsModel) handleSpace() (InputsModel, tea.Cmd) {
	zone, localIdx := m.cursorZone()

	switch zone {
	case zoneFields:
		f := m.fields[localIdx]
		if f.FieldType == FieldToggle {
			val := m.resolveValue(localIdx)
			if val == "true" {
				m.fields[localIdx].Value = "false"
			} else {
				m.fields[localIdx].Value = "true"
			}
			m.textInputs[localIdx].SetValue(m.fields[localIdx].Value)
			return m, nil
		}
	case zoneMCP:
		if localIdx < len(m.mcpServers) {
			m.mcpServers[localIdx].Enabled = !m.mcpServers[localIdx].Enabled
			return m, nil
		}
	}

	return m, nil
}

func (m InputsModel) handleEnter() (InputsModel, tea.Cmd) {
	zone, localIdx := m.cursorZone()
	if zone == zoneFields {
		f := m.fields[localIdx]
		if f.FieldType == FieldToggle {
			return m.handleSpace()
		}
		if f.FieldType == FieldEditor {
			return m.openEditor(localIdx)
		}
	}
	return m, nil
}

func (m InputsModel) openEditor(fieldIdx int) (InputsModel, tea.Cmd) {
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, "forge-extra-context.txt")

	content := m.fields[fieldIdx].Value
	if content == "" {
		content = m.fields[fieldIdx].Default
	}
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		m.flashMsg = fmt.Sprintf("Failed to create temp file: %v", err)
		m.flashErr = true
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearFlashMsg{}
		})
	}

	editor := getEditor()
	c := exec.Command(editor, tmpPath)

	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorDoneMsg{err: err, tmpPath: tmpPath}
	})
}

func (m InputsModel) resolveValue(fieldIdx int) string {
	f := m.fields[fieldIdx]
	if f.Value != "" {
		return f.Value
	}
	return f.Default
}

func (m InputsModel) confirm() (InputsModel, tea.Cmd) {
	// Sync text input values to fields
	for i := range m.fields {
		if m.fields[i].FieldType == FieldText || m.fields[i].FieldType == FieldNumber {
			m.fields[i].Value = m.textInputs[i].Value()
		}
	}

	// Validate
	errs := ValidateSettings(m.fields)
	if len(errs) > 0 {
		m.flashMsg = strings.Join(errs, "; ")
		m.flashErr = true
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return clearFlashMsg{}
		})
	}

	// Build settings
	settings := BuildSettingsFromFields(m.fields, m.mcpServers, m.maxTurns)
	m.state.Settings = settings

	// Write .forge/context.md
	contextContent := generator.GenerateContextFile(m.state)
	contextPath := filepath.Join(m.stateRoot, ".forge", "context.md")
	if err := os.WriteFile(contextPath, []byte(contextContent), 0644); err != nil {
		m.flashMsg = fmt.Sprintf("Failed to write context.md: %v", err)
		m.flashErr = true
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearFlashMsg{}
		})
	}

	// Write CLAUDE.md only if it doesn't exist
	claudeMDPath := filepath.Join(m.stateRoot, "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); os.IsNotExist(err) {
		content := generator.GenerateClaudeMD(m.state)
		if writeErr := os.WriteFile(claudeMDPath, []byte(content), 0644); writeErr != nil {
			m.flashMsg = fmt.Sprintf("Failed to write CLAUDE.md: %v", writeErr)
			m.flashErr = true
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return clearFlashMsg{}
			})
		}
	}

	// Write .claude/settings.json (merge with existing)
	if err := m.writeMCPConfig(); err != nil {
		m.flashMsg = fmt.Sprintf("Failed to write MCP config: %v", err)
		m.flashErr = true
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearFlashMsg{}
		})
	}

	// Save state
	if err := state.Save(m.stateRoot, m.state); err != nil {
		m.flashMsg = fmt.Sprintf("Failed to save state: %v", err)
		m.flashErr = true
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearFlashMsg{}
		})
	}

	return m, func() tea.Msg {
		return TransitionMsg{To: state.PhaseExecution}
	}
}

func (m InputsModel) writeMCPConfig() error {
	// Check if any MCP servers are enabled
	anyEnabled := false
	for _, srv := range m.mcpServers {
		if srv.Enabled {
			anyEnabled = true
			break
		}
	}
	if !anyEnabled {
		return nil
	}

	dir := filepath.Join(m.stateRoot, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "settings.json")

	// Read existing config if present
	var existing map[string]interface{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Get or create mcpServers map
	mcpMap := make(map[string]interface{})
	if existingMCP, ok := existing["mcpServers"].(map[string]interface{}); ok {
		mcpMap = existingMCP
	}

	// Add enabled servers
	for _, srv := range m.mcpServers {
		if srv.Enabled {
			mcpMap[srv.Name] = map[string]interface{}{
				"command": srv.Command,
				"args":    srv.Args,
			}
		}
	}

	existing["mcpServers"] = mcpMap

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (m InputsModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sections []string

	// Header
	stats := computeTaskStatsForInputs(m.state)
	header := lipgloss.NewStyle().
		Foreground(Muted).
		PaddingLeft(1).
		Render(fmt.Sprintf("Plan v%d · %d tasks", m.state.PlanVersion, stats))
	sections = append(sections, header)
	sections = append(sections, "")

	// Render each field
	for i, f := range m.fields {
		active := m.cursor == i
		sections = append(sections, m.renderField(i, f, active))
	}

	// MCP Servers section
	sections = append(sections, "")
	mcpLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(Text).
		PaddingLeft(2).
		Render("MCP Servers")
	sections = append(sections, mcpLabel)

	for i, srv := range m.mcpServers {
		active := m.cursor == len(m.fields)+i
		sections = append(sections, m.renderMCPServer(srv, active))
	}

	// Max Turns display (read-only info)
	sections = append(sections, "")
	turnsLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(Text).
		PaddingLeft(2).
		Render("Max Turns per Task Complexity")
	sections = append(sections, turnsLabel)
	turnsInfo := lipgloss.NewStyle().
		Foreground(Muted).
		PaddingLeft(4).
		Render(fmt.Sprintf("Small: %d    Medium: %d    Large: %d",
			m.maxTurns.Small, m.maxTurns.Medium, m.maxTurns.Large))
	sections = append(sections, turnsInfo)

	// Flash message
	if m.flashMsg != "" {
		sections = append(sections, "")
		color := Success
		if m.flashErr {
			color = Danger
		}
		flash := lipgloss.NewStyle().
			Foreground(color).
			Bold(true).
			PaddingLeft(2).
			Render(m.flashMsg)
		sections = append(sections, flash)
	}

	// Footer help
	sections = append(sections, "")
	help := HelpStyle.Render(
		"Tab/Shift+Tab navigate · Enter edit · Space toggle · c confirm · b back · q quit")
	sections = append(sections, help)

	content := strings.Join(sections, "\n")

	// Ensure content fits
	return lipgloss.NewStyle().
		Width(m.width).
		MaxHeight(m.height).
		Render(content)
}

func (m InputsModel) renderField(idx int, f InputField, active bool) string {
	var lines []string

	// Label
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Text).
		PaddingLeft(2)
	if active {
		labelStyle = labelStyle.Foreground(Secondary)
	}
	lines = append(lines, labelStyle.Render(f.Label))

	// Value display
	switch f.FieldType {
	case FieldText, FieldNumber:
		inputView := m.textInputs[idx].View()
		boxStyle := lipgloss.NewStyle().PaddingLeft(2)
		lines = append(lines, boxStyle.Render(inputView))

	case FieldToggle:
		val := m.resolveValue(idx)
		checkbox := "[ ] No"
		if val == "true" {
			checkbox = "[x] Yes"
		}
		toggleStyle := lipgloss.NewStyle().PaddingLeft(4)
		if active {
			toggleStyle = toggleStyle.Foreground(Secondary)
		}
		lines = append(lines, toggleStyle.Render(checkbox))

	case FieldEditor:
		val := m.resolveValue(idx)
		display := val
		if display == "" {
			display = "(empty — press Enter to open editor)"
		} else if len(display) > 60 {
			display = display[:57] + "..."
		}
		editorStyle := lipgloss.NewStyle().PaddingLeft(4).Foreground(Muted)
		if active {
			editorStyle = editorStyle.Foreground(Secondary)
		}
		lines = append(lines, editorStyle.Render(display))
	}

	// Help text
	if f.HelpText != "" {
		helpStyle := lipgloss.NewStyle().Foreground(Muted).PaddingLeft(4)
		lines = append(lines, helpStyle.Render(f.HelpText))
	}

	return strings.Join(lines, "\n")
}

func (m InputsModel) renderMCPServer(srv MCPServer, active bool) string {
	checkbox := "[ ]"
	if srv.Enabled {
		checkbox = "[x]"
	}

	style := lipgloss.NewStyle().PaddingLeft(4)
	if active {
		style = style.Foreground(Secondary)
	}

	descStyle := lipgloss.NewStyle().Foreground(Muted)
	line := fmt.Sprintf("%s %s — %s", checkbox, srv.Name, descStyle.Render(srv.Description))
	return style.Render(line)
}

func computeTaskStatsForInputs(s *state.State) int {
	count := 0
	for _, t := range s.Tasks {
		if t.Status != state.TaskCancelled && t.Status != state.TaskSkipped {
			count++
		}
	}
	return count
}

func (m *InputsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	// Update text input widths
	inputWidth := w - 8
	if inputWidth < 20 {
		inputWidth = 20
	}
	for i := range m.textInputs {
		m.textInputs[i].Width = inputWidth
	}
}
