package claude

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Response represents a parsed response from Claude Code CLI.
type Response struct {
	Text      string // the assistant's text response
	SessionID string // for --continue support
	RawJSON   string // raw JSON output from --output-format json
}

// StreamCallback is called with each text chunk as it arrives from Claude.
// It's called from a goroutine — implementations must be goroutine-safe.
type StreamCallback func(chunk string)

// Claude defines the interface for interacting with Claude Code CLI.
// Production code uses Client. Tests use MockClaude.
type Claude interface {
	Send(ctx context.Context, prompt string) (*Response, error)
	Continue(ctx context.Context, message string) (*Response, error)
	SendStreaming(ctx context.Context, prompt string, onChunk StreamCallback) (*Response, error)
	ContinueStreaming(ctx context.Context, message string, onChunk StreamCallback) (*Response, error)
}

// Verify Client implements Claude at compile time.
var _ Claude = (*Client)(nil)

// Client wraps the claude CLI.
type Client struct {
	claudePath string
	timeout    time.Duration
	model      string // model to use (e.g., "sonnet", "opus")
	maxTurns   int    // max turns per invocation (default 1 for planning)
	envVars    map[string]string // environment variables to pass to the CLI
}

// NewClient creates a new Claude Code CLI client.
// Verifies the claude binary exists at the given path (or "claude" if empty).
// model defaults to "sonnet" if empty. maxTurns defaults to 1.
func NewClient(claudePath string, timeout time.Duration, model string) (*Client, error) {
	if claudePath == "" {
		claudePath = "claude"
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	if model == "" {
		model = "sonnet"
	}

	path, err := exec.LookPath(claudePath)
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found at %q — install it from https://docs.anthropic.com/en/docs/claude-code", claudePath)
	}

	return &Client{
		claudePath: path,
		timeout:    timeout,
		model:      model,
		maxTurns:   1,
		envVars:    make(map[string]string),
	}, nil
}

// WithModel returns a copy of the client with a different model.
func (c *Client) WithModel(model string) *Client {
	clone := *c
	clone.model = model
	return &clone
}

// WithMaxTurns returns a copy of the client with a different max-turns setting.
func (c *Client) WithMaxTurns(n int) *Client {
	clone := *c
	clone.maxTurns = n
	return &clone
}

// WithEnvVars returns a copy of the client with environment variables.
func (c *Client) WithEnvVars(envVars map[string]string) *Client {
	clone := *c
	clone.envVars = make(map[string]string)
	for k, v := range envVars {
		clone.envVars[k] = v
	}
	return &clone
}

// Send sends a one-shot message to Claude Code (non-streaming).
// Runs: claude -p "<prompt>" --output-format json
func (c *Client) Send(ctx context.Context, prompt string) (*Response, error) {
	args := []string{"-p", prompt, "--output-format", "json"}
	return c.runClaude(ctx, args)
}

// Continue sends a follow-up message in an existing session (non-streaming).
// Runs: claude -p "<message>" --continue --output-format json
func (c *Client) Continue(ctx context.Context, message string) (*Response, error) {
	args := []string{"-p", message, "--continue", "--output-format", "json"}
	return c.runClaude(ctx, args)
}

// SendWithSystem sends a message with a separate system prompt (non-streaming).
// Constructs the prompt as: system prompt + "\n\n" + user message.
func (c *Client) SendWithSystem(ctx context.Context, systemPrompt, userMessage string) (*Response, error) {
	prompt := systemPrompt + "\n\n" + userMessage
	return c.Send(ctx, prompt)
}

// SendStreaming sends a one-shot prompt and streams the response.
// onChunk is called with each text fragment as it arrives.
// Returns the complete assembled response when the stream ends.
func (c *Client) SendStreaming(ctx context.Context, prompt string, onChunk StreamCallback) (*Response, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--max-turns", fmt.Sprintf("%d", c.maxTurns),
		"--model", c.model,
	}
	return c.runClaudeStreaming(ctx, args, onChunk)
}

// ContinueStreaming sends a follow-up message in the existing session and streams the response.
func (c *Client) ContinueStreaming(ctx context.Context, message string, onChunk StreamCallback) (*Response, error) {
	args := []string{
		"-p", message,
		"--continue",
		"--output-format", "stream-json",
		"--max-turns", fmt.Sprintf("%d", c.maxTurns),
		"--model", c.model,
	}
	return c.runClaudeStreaming(ctx, args, onChunk)
}

func (c *Client) runClaude(ctx context.Context, args []string) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.claudePath, args...)

	// Set environment variables if provided
	if len(c.envVars) > 0 {
		env := os.Environ()
		for k, v := range c.envVars {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("claude timed out after %v", c.timeout)
		}
		return nil, fmt.Errorf("claude failed: %w\nstderr: %s", err, stderr.String())
	}

	return parseResponse(stdout.Bytes())
}

func (c *Client) runClaudeStreaming(ctx context.Context, args []string, onChunk StreamCallback) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.claudePath, args...)

	// Set environment variables if provided
	if len(c.envVars) > 0 {
		env := os.Environ()
		for k, v := range c.envVars {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Read streaming JSON output line by line
	var fullText strings.Builder
	scanner := bufio.NewScanner(stdout)

	// Increase scanner buffer for long lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		chunk := parseStreamChunk(line)
		if chunk != "" {
			fullText.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("error reading claude output: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("claude timed out after %v", c.timeout)
		}
		return nil, fmt.Errorf("claude failed: %w\nstderr: %s", err, stderr.String())
	}

	return &Response{
		Text: fullText.String(),
	}, nil
}
