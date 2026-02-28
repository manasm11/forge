package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/manasm11/forge/internal/provider"
)

// providerSelectModel is a minimal bubbletea model for inline provider selection.
type providerSelectModel struct {
	cursor       int // 0=Claude, 1=Ollama
	choice       provider.ProviderType
	confirmed    bool
	quit         bool
	ollamaStatus provider.OllamaStatus
	width        int
}

func newProviderSelectModel(ollamaStatus provider.OllamaStatus) providerSelectModel {
	return providerSelectModel{
		ollamaStatus: ollamaStatus,
		width:        50,
	}
}

func (m providerSelectModel) Init() tea.Cmd {
	return nil
}

func (m providerSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = 0
		case "down", "j":
			m.cursor = 1
		case "1":
			m.choice = provider.ProviderAnthropic
			m.confirmed = true
			return m, tea.Quit
		case "2":
			m.choice = provider.ProviderOllama
			m.confirmed = true
			return m, tea.Quit
		case "enter", " ":
			if m.cursor == 0 {
				m.choice = provider.ProviderAnthropic
			} else {
				m.choice = provider.ProviderOllama
			}
			m.confirmed = true
			return m, tea.Quit
		case "q", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m providerSelectModel) View() string {
	if m.confirmed {
		name := "Claude (cloud)"
		if m.choice == provider.ProviderOllama {
			name = "Ollama (local)"
		}
		done := lipgloss.NewStyle().Foreground(Success).Render("  ✓ Selected " + name + " provider")
		return done + "\n"
	}

	if m.quit {
		return ""
	}

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		Render("  ⚒ forge — Select Provider")

	// Build option lines
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(Secondary)
	normalStyle := lipgloss.NewStyle().Foreground(Text)
	subtitleStyle := lipgloss.NewStyle().Foreground(Muted)

	var lines []string
	lines = append(lines, "")

	// Claude option
	if m.cursor == 0 {
		lines = append(lines, selectedStyle.Render("  ▸ ☁  Claude (cloud)"))
		lines = append(lines, subtitleStyle.Render("       Standard Claude Code CLI"))
	} else {
		lines = append(lines, normalStyle.Render("    ☁  Claude (cloud)"))
		lines = append(lines, subtitleStyle.Render("       Standard Claude Code CLI"))
	}

	lines = append(lines, "")

	// Ollama option with status info
	ollamaSub := "Local execution"
	if m.ollamaStatus.Available {
		var parts []string
		parts = append(parts, "Local execution")
		if len(m.ollamaStatus.Models) > 0 {
			parts = append(parts, fmt.Sprintf("%d models", len(m.ollamaStatus.Models)))
		}
		if m.ollamaStatus.Version != "" {
			parts = append(parts, m.ollamaStatus.Version)
		}
		ollamaSub = strings.Join(parts, " · ")
	}

	if m.cursor == 1 {
		lines = append(lines, selectedStyle.Render("  ▸ 🖥  Ollama (local)"))
		lines = append(lines, subtitleStyle.Render("       "+ollamaSub))
	} else {
		lines = append(lines, normalStyle.Render("    🖥  Ollama (local)"))
		lines = append(lines, subtitleStyle.Render("       "+ollamaSub))
	}

	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	// Box
	boxWidth := 46
	if m.width > 10 && m.width-6 > boxWidth {
		boxWidth = m.width - 6
		if boxWidth > 60 {
			boxWidth = 60
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Width(boxWidth).
		PaddingLeft(1).
		PaddingRight(1).
		Render(content)

	// Help
	help := lipgloss.NewStyle().
		Foreground(Muted).
		Render("  ↑/↓ navigate · enter confirm · q quit")

	return fmt.Sprintf("\n%s\n\n%s\n\n%s\n", title, box, help)
}

// RunProviderSelection runs an inline bubbletea program for provider selection.
// Returns the chosen provider type, or an error if the user quit without selecting.
func RunProviderSelection(ollamaStatus provider.OllamaStatus) (provider.ProviderType, error) {
	m := newProviderSelectModel(ollamaStatus)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("provider selection failed: %w", err)
	}

	result := finalModel.(providerSelectModel)
	if !result.confirmed {
		return "", fmt.Errorf("provider selection cancelled")
	}

	return result.choice, nil
}
