package components

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseSlashCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    SlashCommand
		wantOK  bool
	}{
		{
			name:   "done command",
			input:  "/done",
			want:   SlashCommand{Name: "done", Args: ""},
			wantOK: true,
		},
		{
			name:   "summary command",
			input:  "/summary",
			want:   SlashCommand{Name: "summary", Args: ""},
			wantOK: true,
		},
		{
			name:   "done with args",
			input:  "/done now",
			want:   SlashCommand{Name: "done", Args: "now"},
			wantOK: true,
		},
		{
			name:   "regular text",
			input:  "hello",
			want:   SlashCommand{},
			wantOK: false,
		},
		{
			name:   "just slash",
			input:  "/",
			want:   SlashCommand{},
			wantOK: false,
		},
		{
			name:   "unknown command with args",
			input:  "/unknown command with args",
			want:   SlashCommand{Name: "unknown", Args: "command with args"},
			wantOK: true,
		},
		{
			name:   "command with extra spaces in args",
			input:  "/restart  extra  spaces",
			want:   SlashCommand{Name: "restart", Args: "extra  spaces"},
			wantOK: true,
		},
		{
			name:   "empty string",
			input:  "",
			want:   SlashCommand{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseSlashCommand(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseSlashCommand(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got.Name != tt.want.Name {
				t.Errorf("ParseSlashCommand(%q) Name = %q, want %q", tt.input, got.Name, tt.want.Name)
			}
			if got.Args != tt.want.Args {
				t.Errorf("ParseSlashCommand(%q) Args = %q, want %q", tt.input, got.Args, tt.want.Args)
			}
		})
	}
}

func TestNewChatModel(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	handler := func(cmd SlashCommand) (tea.Cmd, bool) { return nil, false }

	m := NewChatModel(sender, handler)

	if len(m.messages) != 0 {
		t.Errorf("NewChatModel() messages = %d, want 0", len(m.messages))
	}
	if m.waiting {
		t.Error("NewChatModel() waiting = true, want false")
	}
	if !m.textInput.Focused() {
		t.Error("NewChatModel() textInput not focused")
	}
}

func TestAddMessage(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	m.AddMessage(RoleUser, "hello")
	m.AddMessage(RoleAssistant, "hi there")
	m.AddMessage(RoleSystem, "status update")

	msgs := m.Messages()
	if len(msgs) != 3 {
		t.Fatalf("Messages() len = %d, want 3", len(msgs))
	}
	if msgs[0].Role != RoleUser || msgs[0].Content != "hello" {
		t.Errorf("Messages()[0] = {%s, %q}, want {user, \"hello\"}", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != RoleAssistant || msgs[1].Content != "hi there" {
		t.Errorf("Messages()[1] = {%s, %q}, want {assistant, \"hi there\"}", msgs[1].Role, msgs[1].Content)
	}
	if msgs[2].Role != RoleSystem || msgs[2].Content != "status update" {
		t.Errorf("Messages()[2] = {%s, %q}, want {system, \"status update\"}", msgs[2].Role, msgs[2].Content)
	}
}

func TestMessagesReturnsCopy(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	m.AddMessage(RoleUser, "original")

	msgs := m.Messages()
	msgs[0].Content = "modified"

	// Internal state should be unchanged
	internal := m.Messages()
	if internal[0].Content != "original" {
		t.Errorf("Messages() did not return a copy: internal content = %q, want \"original\"", internal[0].Content)
	}
}

func TestIsWaiting(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)

	if m.IsWaiting() {
		t.Error("IsWaiting() = true before any message sent, want false")
	}
}

func TestClearMessages(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	m.AddMessage(RoleUser, "hello")
	m.AddMessage(RoleAssistant, "hi")

	m.ClearMessages()

	if len(m.Messages()) != 0 {
		t.Errorf("ClearMessages() messages = %d, want 0", len(m.Messages()))
	}
}

func TestStreamStart(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	m, _ = m.Update(StreamStartMsg{})

	if !m.streaming {
		t.Error("StreamStartMsg should set streaming=true")
	}
	if !m.waiting {
		t.Error("StreamStartMsg should set waiting=true")
	}
	msgs := m.Messages()
	if len(msgs) != 1 {
		t.Fatalf("should have 1 message (empty assistant), got %d", len(msgs))
	}
	if msgs[0].Role != RoleAssistant {
		t.Errorf("message role = %q, want %q", msgs[0].Role, RoleAssistant)
	}
	if msgs[0].Content != "" {
		t.Errorf("streaming message content should be empty, got %q", msgs[0].Content)
	}
}

func TestStreamChunk(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	// Start stream first
	m, _ = m.Update(StreamStartMsg{})

	// Send chunks
	m, _ = m.Update(StreamChunkMsg{Chunk: "Hello "})
	m, _ = m.Update(StreamChunkMsg{Chunk: "world!"})

	msgs := m.Messages()
	if len(msgs) != 1 {
		t.Fatalf("should still have 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Hello world!" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "Hello world!")
	}
}

func TestStreamDone(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	// Start and complete stream
	m, _ = m.Update(StreamStartMsg{})
	m, _ = m.Update(StreamChunkMsg{Chunk: "response text"})
	m, _ = m.Update(StreamDoneMsg{FullText: "response text"})

	if m.streaming {
		t.Error("StreamDoneMsg should set streaming=false")
	}
	if m.waiting {
		t.Error("StreamDoneMsg should set waiting=false")
	}
}

func TestStreamDone_WithError(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)
	m.SetSize(80, 24)

	// Start stream (creates empty assistant message)
	m, _ = m.Update(StreamStartMsg{})

	// Complete with error — empty streaming message should become error
	m, _ = m.Update(StreamDoneMsg{Err: fmt.Errorf("connection failed")})

	msgs := m.Messages()
	if len(msgs) != 1 {
		t.Fatalf("should have 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != RoleSystem {
		t.Errorf("error message role = %q, want %q", msgs[0].Role, RoleSystem)
	}
	if msgs[0].Content != "Error: connection failed" {
		t.Errorf("error message content = %q", msgs[0].Content)
	}
}

func TestSetSize(t *testing.T) {
	t.Parallel()
	sender := func(text string) tea.Cmd { return nil }
	m := NewChatModel(sender, nil)

	m.SetSize(100, 40)

	if m.width != 100 {
		t.Errorf("SetSize() width = %d, want 100", m.width)
	}
	if m.height != 40 {
		t.Errorf("SetSize() height = %d, want 40", m.height)
	}
	if !m.ready {
		t.Error("SetSize() ready = false, want true")
	}
	// viewport height should be total height minus input area
	expectedVPHeight := 40 - inputAreaHeight
	if m.viewport.Height != expectedVPHeight {
		t.Errorf("SetSize() viewport.Height = %d, want %d", m.viewport.Height, expectedVPHeight)
	}
}
