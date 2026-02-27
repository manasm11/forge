package executor

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// RealTestRunner implements TestRunner using real command execution.
type RealTestRunner struct {
	dir string
}

// NewRealTestRunner creates a TestRunner rooted at dir.
func NewRealTestRunner(dir string) *RealTestRunner {
	return &RealTestRunner{dir: dir}
}

func (r *RealTestRunner) runCommand(ctx context.Context, command string) *TestResult {
	start := time.Now()

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return &TestResult{Passed: true, Output: "no command"}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()

	result := &TestResult{
		Output:   string(out),
		Duration: time.Since(start).Seconds(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Passed = false
	} else {
		result.Passed = true
		result.ExitCode = 0
	}

	return result
}

func (r *RealTestRunner) RunTests(ctx context.Context, command string) *TestResult {
	return r.runCommand(ctx, command)
}

func (r *RealTestRunner) RunBuild(ctx context.Context, command string) *TestResult {
	return r.runCommand(ctx, command)
}
