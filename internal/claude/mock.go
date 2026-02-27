package claude

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// MockClaude is a test mock that returns predefined responses.
type MockClaude struct {
	// Responses is a queue of responses to return. Each call pops the next one.
	Responses []MockResponse
	// Calls records every call made for assertion.
	Calls   []MockCall
	callIdx int
}

// MockResponse defines the response for a single call.
type MockResponse struct {
	Text string
	Err  error
	// Chunks simulates streaming — each string is sent as a separate chunk.
	// If nil, the full Text is sent as a single chunk.
	Chunks []string
}

// MockCall records a single call to a mock method.
type MockCall struct {
	Method string // "Send", "Continue", "SendStreaming", "ContinueStreaming"
	Prompt string
}

// Verify MockClaude implements Claude at compile time.
var _ Claude = (*MockClaude)(nil)

// NewMockClaude creates a mock with the given response queue.
func NewMockClaude(responses ...MockResponse) *MockClaude {
	return &MockClaude{
		Responses: responses,
	}
}

func (m *MockClaude) nextResponse() MockResponse {
	if m.callIdx >= len(m.Responses) {
		return MockResponse{Err: fmt.Errorf("mock: no more responses (call %d)", m.callIdx)}
	}
	resp := m.Responses[m.callIdx]
	m.callIdx++
	return resp
}

func (m *MockClaude) Send(_ context.Context, prompt string) (*Response, error) {
	m.Calls = append(m.Calls, MockCall{Method: "Send", Prompt: prompt})
	resp := m.nextResponse()
	if resp.Err != nil {
		return nil, resp.Err
	}
	return &Response{Text: resp.Text}, nil
}

func (m *MockClaude) Continue(_ context.Context, message string) (*Response, error) {
	m.Calls = append(m.Calls, MockCall{Method: "Continue", Prompt: message})
	resp := m.nextResponse()
	if resp.Err != nil {
		return nil, resp.Err
	}
	return &Response{Text: resp.Text}, nil
}

func (m *MockClaude) SendStreaming(_ context.Context, prompt string, onChunk StreamCallback) (*Response, error) {
	m.Calls = append(m.Calls, MockCall{Method: "SendStreaming", Prompt: prompt})
	resp := m.nextResponse()
	if resp.Err != nil {
		return nil, resp.Err
	}

	// Simulate streaming
	if resp.Chunks != nil {
		for _, chunk := range resp.Chunks {
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	} else if onChunk != nil {
		onChunk(resp.Text)
	}

	return &Response{Text: resp.Text}, nil
}

func (m *MockClaude) ContinueStreaming(_ context.Context, message string, onChunk StreamCallback) (*Response, error) {
	m.Calls = append(m.Calls, MockCall{Method: "ContinueStreaming", Prompt: message})
	resp := m.nextResponse()
	if resp.Err != nil {
		return nil, resp.Err
	}

	// Simulate streaming
	if resp.Chunks != nil {
		for _, chunk := range resp.Chunks {
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	} else if onChunk != nil {
		onChunk(resp.Text)
	}

	return &Response{Text: resp.Text}, nil
}

// AssertCallCount verifies the expected number of calls were made.
func (m *MockClaude) AssertCallCount(t *testing.T, expected int) {
	t.Helper()
	if len(m.Calls) != expected {
		t.Errorf("MockClaude: call count = %d, want %d", len(m.Calls), expected)
	}
}

// AssertCall verifies a specific call's method and that the prompt contains a substring.
func (m *MockClaude) AssertCall(t *testing.T, index int, method string, promptContains string) {
	t.Helper()
	if index >= len(m.Calls) {
		t.Fatalf("MockClaude: call index %d out of range (have %d calls)", index, len(m.Calls))
	}
	call := m.Calls[index]
	if call.Method != method {
		t.Errorf("MockClaude: call[%d].Method = %q, want %q", index, call.Method, method)
	}
	if promptContains != "" && !strings.Contains(call.Prompt, promptContains) {
		t.Errorf("MockClaude: call[%d].Prompt does not contain %q\ngot: %s", index, promptContains, call.Prompt)
	}
}
