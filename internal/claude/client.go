package claude

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Response represents a parsed response from Claude Code CLI.
type Response struct {
	Text      string // the assistant's text response
	SessionID string // for --continue support
	RawJSON   string // raw JSON output from --output-format json
}

// Client wraps the claude CLI.
type Client struct {
	claudePath string
	timeout    time.Duration
}

// NewClient creates a new Claude Code CLI client.
// Verifies the claude binary exists at the given path (or "claude" if empty).
func NewClient(claudePath string, timeout time.Duration) (*Client, error) {
	if claudePath == "" {
		claudePath = "claude"
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	path, err := exec.LookPath(claudePath)
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found at %q — install it from https://docs.anthropic.com/en/docs/claude-code", claudePath)
	}

	return &Client{
		claudePath: path,
		timeout:    timeout,
	}, nil
}

// Send sends a one-shot message to Claude Code.
// Runs: claude -p "<prompt>" --output-format json
func (c *Client) Send(ctx context.Context, prompt string) (*Response, error) {
	args := []string{"-p", prompt, "--output-format", "json"}
	return c.runClaude(ctx, args)
}

// Continue sends a follow-up message in an existing session.
// Runs: claude -p "<message>" --continue --output-format json
func (c *Client) Continue(ctx context.Context, message string) (*Response, error) {
	args := []string{"-p", message, "--continue", "--output-format", "json"}
	return c.runClaude(ctx, args)
}

// SendWithSystem sends a message with a separate system prompt.
// Constructs the prompt as: system prompt + "\n\n" + user message.
func (c *Client) SendWithSystem(ctx context.Context, systemPrompt, userMessage string) (*Response, error) {
	prompt := systemPrompt + "\n\n" + userMessage
	return c.Send(ctx, prompt)
}

func (c *Client) runClaude(ctx context.Context, args []string) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.claudePath, args...)

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
