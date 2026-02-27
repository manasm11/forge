package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RealClaudeExecutor implements ClaudeExecutor using the claude CLI.
type RealClaudeExecutor struct {
	dir string
}

// NewRealClaudeExecutor creates a ClaudeExecutor rooted at dir.
func NewRealClaudeExecutor(dir string) *RealClaudeExecutor {
	return &RealClaudeExecutor{dir: dir}
}

func (e *RealClaudeExecutor) Execute(ctx context.Context, opts ExecuteOpts) (*ExecuteResult, error) {
	args := []string{
		"--print",
		"--output-format", "stream-json",
	}

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}

	args = append(args, "--prompt", opts.Prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	} else {
		cmd.Dir = e.dir
	}

	if len(opts.EnvVars) > 0 {
		cmd.Env = append(os.Environ(), mapToEnv(opts.EnvVars)...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	var fullText strings.Builder
	scanner := bufio.NewScanner(stdout)
	// Set a larger buffer for potentially large output lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		text := parseStreamChunk(line)
		if text != "" {
			fullText.WriteString(text)
			if opts.OnChunk != nil {
				opts.OnChunk(text)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("claude exited with error: %w", err)
	}

	return &ExecuteResult{
		Text:     fullText.String(),
		Duration: time.Since(start).Seconds(),
	}, nil
}

// parseStreamChunk extracts displayable text from a single line of stream-json output.
func parseStreamChunk(line string) string {
	if line == "" {
		return ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return ""
	}

	msgType, _ := obj["type"].(string)

	// content_block_delta with text_delta
	if delta, ok := obj["delta"].(map[string]interface{}); ok {
		if text, ok := delta["text"].(string); ok {
			return text
		}
	}

	// assistant message with content array
	if msg, ok := obj["message"].(map[string]interface{}); ok {
		if content, ok := msg["content"].([]interface{}); ok {
			var texts []string
			for _, block := range content {
				if b, ok := block.(map[string]interface{}); ok {
					if text, ok := b["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
			return strings.Join(texts, "")
		}
	}

	// result type — skip to avoid duplication
	if msgType == "result" {
		return ""
	}

	// content_block_start with initial text
	if cb, ok := obj["content_block"].(map[string]interface{}); ok {
		if text, ok := cb["text"].(string); ok && text != "" {
			return text
		}
	}

	return ""
}

func mapToEnv(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}
