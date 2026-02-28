package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manasm11/forge/internal/provider"
)

func TestProviderSelectNavigation(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantCursor int
	}{
		{"initial cursor is 0", nil, 0},
		{"down moves to 1", []string{"down"}, 1},
		{"j moves to 1", []string{"j"}, 1},
		{"up stays at 0", []string{"up"}, 0},
		{"k stays at 0", []string{"k"}, 0},
		{"down then up", []string{"down", "up"}, 0},
		{"down down stays at 1", []string{"down", "down"}, 1},
		{"j then k", []string{"j", "k"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newProviderSelectModel(provider.OllamaStatus{})
			var model tea.Model = m

			for _, key := range tt.keys {
				model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			}

			result := model.(providerSelectModel)
			if result.cursor != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", result.cursor, tt.wantCursor)
			}
		})
	}
}

func TestProviderSelectConfirm(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantChoice provider.ProviderType
	}{
		{"enter on Claude", []string{"enter"}, provider.ProviderAnthropic},
		{"enter on Ollama", []string{"down", "enter"}, provider.ProviderOllama},
		{"space on Claude", []string{" "}, provider.ProviderAnthropic},
		{"space on Ollama", []string{"j", " "}, provider.ProviderOllama},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newProviderSelectModel(provider.OllamaStatus{})
			var model tea.Model = m

			for _, key := range tt.keys {
				model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			}

			result := model.(providerSelectModel)
			if !result.confirmed {
				t.Fatal("expected confirmed=true")
			}
			if result.choice != tt.wantChoice {
				t.Errorf("choice = %q, want %q", result.choice, tt.wantChoice)
			}
		})
	}
}

func TestProviderSelectDirectKeys(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantChoice provider.ProviderType
	}{
		{"1 selects Claude", "1", provider.ProviderAnthropic},
		{"2 selects Ollama", "2", provider.ProviderOllama},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newProviderSelectModel(provider.OllamaStatus{})
			model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			result := model.(providerSelectModel)
			if !result.confirmed {
				t.Fatal("expected confirmed=true")
			}
			if result.choice != tt.wantChoice {
				t.Errorf("choice = %q, want %q", result.choice, tt.wantChoice)
			}
		})
	}
}

func TestProviderSelectQuit(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{"q quits", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
		{"ctrl+c quits", tea.KeyMsg{Type: tea.KeyCtrlC}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newProviderSelectModel(provider.OllamaStatus{})
			model, _ := m.Update(tt.msg)

			result := model.(providerSelectModel)
			if result.confirmed {
				t.Fatal("expected confirmed=false on quit")
			}
			if !result.quit {
				t.Fatal("expected quit=true")
			}
		})
	}
}

func TestProviderSelectView(t *testing.T) {
	t.Run("shows both options", func(t *testing.T) {
		m := newProviderSelectModel(provider.OllamaStatus{})
		view := m.View()

		if !strings.Contains(view, "Claude (cloud)") {
			t.Error("view should contain 'Claude (cloud)'")
		}
		if !strings.Contains(view, "Ollama (local)") {
			t.Error("view should contain 'Ollama (local)'")
		}
		if !strings.Contains(view, "Select Provider") {
			t.Error("view should contain 'Select Provider'")
		}
		if !strings.Contains(view, "navigate") {
			t.Error("view should contain help text")
		}
	})

	t.Run("shows cursor on selected item", func(t *testing.T) {
		m := newProviderSelectModel(provider.OllamaStatus{})
		view := m.View()
		if !strings.Contains(view, "▸") {
			t.Error("view should contain cursor '▸'")
		}
	})

	t.Run("confirmed view shows checkmark", func(t *testing.T) {
		m := newProviderSelectModel(provider.OllamaStatus{})
		m.confirmed = true
		m.choice = provider.ProviderAnthropic
		view := m.View()

		if !strings.Contains(view, "✓") {
			t.Error("confirmed view should contain checkmark")
		}
		if !strings.Contains(view, "Claude (cloud)") {
			t.Error("confirmed view should mention Claude")
		}
	})

	t.Run("confirmed Ollama shows Ollama", func(t *testing.T) {
		m := newProviderSelectModel(provider.OllamaStatus{})
		m.confirmed = true
		m.choice = provider.ProviderOllama
		view := m.View()

		if !strings.Contains(view, "Ollama (local)") {
			t.Error("confirmed view should mention Ollama")
		}
	})

	t.Run("quit view is empty", func(t *testing.T) {
		m := newProviderSelectModel(provider.OllamaStatus{})
		m.quit = true
		view := m.View()
		if view != "" {
			t.Errorf("quit view should be empty, got %q", view)
		}
	})
}

func TestProviderSelectOllamaInfo(t *testing.T) {
	t.Run("shows model count and version", func(t *testing.T) {
		status := provider.OllamaStatus{
			Available: true,
			Version:   "v0.5.1",
			Models: []provider.OllamaModel{
				{Name: "qwen3-coder:latest"},
				{Name: "llama3:latest"},
				{Name: "mistral:latest"},
			},
		}
		m := newProviderSelectModel(status)
		view := m.View()

		if !strings.Contains(view, "3 models") {
			t.Error("view should show model count")
		}
		if !strings.Contains(view, "v0.5.1") {
			t.Error("view should show version")
		}
	})

	t.Run("no models or version", func(t *testing.T) {
		status := provider.OllamaStatus{
			Available: true,
		}
		m := newProviderSelectModel(status)
		view := m.View()

		if !strings.Contains(view, "Local execution") {
			t.Error("view should show 'Local execution'")
		}
	})
}

func TestProviderSelectWindowSize(t *testing.T) {
	m := newProviderSelectModel(provider.OllamaStatus{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	result := model.(providerSelectModel)
	if result.width != 80 {
		t.Errorf("width = %d, want 80", result.width)
	}
}
