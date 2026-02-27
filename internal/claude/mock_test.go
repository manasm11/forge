package claude

import (
	"context"
	"fmt"
	"testing"
)

func TestMockClaude_Send(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(MockResponse{Text: "hello back"})

	resp, err := mock.Send(context.Background(), "hello")

	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp.Text != "hello back" {
		t.Errorf("resp.Text = %q, want %q", resp.Text, "hello back")
	}
	mock.AssertCallCount(t, 1)
	mock.AssertCall(t, 0, "Send", "hello")
}

func TestMockClaude_Continue(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(MockResponse{Text: "continued"})

	resp, err := mock.Continue(context.Background(), "follow up")

	if err != nil {
		t.Fatalf("Continue() error: %v", err)
	}
	if resp.Text != "continued" {
		t.Errorf("resp.Text = %q, want %q", resp.Text, "continued")
	}
	mock.AssertCallCount(t, 1)
	mock.AssertCall(t, 0, "Continue", "follow up")
}

func TestMockClaude_SendStreaming(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(MockResponse{
		Text:   "hello world",
		Chunks: []string{"hello ", "world"},
	})

	var chunks []string
	resp, err := mock.SendStreaming(context.Background(), "prompt", func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("SendStreaming() error: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("resp.Text = %q", resp.Text)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks count = %d, want 2", len(chunks))
	}
	if chunks[0] != "hello " || chunks[1] != "world" {
		t.Errorf("chunks = %v", chunks)
	}
	mock.AssertCall(t, 0, "SendStreaming", "prompt")
}

func TestMockClaude_SendStreaming_NoChunks(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(MockResponse{Text: "full text"})

	var chunks []string
	resp, err := mock.SendStreaming(context.Background(), "prompt", func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("SendStreaming() error: %v", err)
	}
	if resp.Text != "full text" {
		t.Errorf("resp.Text = %q", resp.Text)
	}
	if len(chunks) != 1 || chunks[0] != "full text" {
		t.Errorf("expected single chunk with full text, got %v", chunks)
	}
}

func TestMockClaude_Error(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(MockResponse{Err: fmt.Errorf("connection failed")})

	resp, err := mock.Send(context.Background(), "hello")

	if err == nil {
		t.Fatal("expected error")
	}
	if resp != nil {
		t.Errorf("resp should be nil on error, got %v", resp)
	}
}

func TestMockClaude_ExhaustedResponses(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(MockResponse{Text: "only one"})

	_, _ = mock.Send(context.Background(), "first")
	_, err := mock.Send(context.Background(), "second")

	if err == nil {
		t.Fatal("expected error when responses exhausted")
	}
}

func TestMockClaude_MultipleCallsSequence(t *testing.T) {
	t.Parallel()
	mock := NewMockClaude(
		MockResponse{Text: "first response"},
		MockResponse{Text: "second response"},
		MockResponse{Text: "third response"},
	)

	resp1, _ := mock.Send(context.Background(), "msg1")
	resp2, _ := mock.Continue(context.Background(), "msg2")
	resp3, _ := mock.ContinueStreaming(context.Background(), "msg3", nil)

	if resp1.Text != "first response" {
		t.Errorf("resp1.Text = %q", resp1.Text)
	}
	if resp2.Text != "second response" {
		t.Errorf("resp2.Text = %q", resp2.Text)
	}
	if resp3.Text != "third response" {
		t.Errorf("resp3.Text = %q", resp3.Text)
	}

	mock.AssertCallCount(t, 3)
	mock.AssertCall(t, 0, "Send", "msg1")
	mock.AssertCall(t, 1, "Continue", "msg2")
	mock.AssertCall(t, 2, "ContinueStreaming", "msg3")
}
