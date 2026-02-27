package preflight

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

type CheckResult struct {
	Name    string
	Found   bool
	Version string
	Error   string
}

var requiredTools = []string{"claude", "gh", "git"}

// RunAll checks for required external tools and returns results.
// Required: claude, gh, git
func RunAll() []CheckResult {
	results := make([]CheckResult, len(requiredTools))
	for i, tool := range requiredTools {
		results[i] = check(tool)
	}
	return results
}

func check(name string) CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			errMsg = "timed out checking version"
		}
		return CheckResult{
			Name:  name,
			Found: false,
			Error: errMsg,
		}
	}

	version := strings.TrimSpace(string(out))
	// Take only the first line if there are multiple
	if idx := strings.IndexByte(version, '\n'); idx != -1 {
		version = version[:idx]
	}

	return CheckResult{
		Name:    name,
		Found:   true,
		Version: version,
	}
}
