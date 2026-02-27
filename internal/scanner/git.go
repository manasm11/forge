package scanner

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

const gitTimeout = 5 * time.Second

// scanGit gathers git repository information.
// Returns empty/zero values if not a git repo or git is not installed.
func scanGit(root string) (branch string, dirty bool, commits []string) {
	// Check if git is available and this is a git repo
	if !isGitRepo(root) {
		return
	}

	branch = runGit(root, "rev-parse", "--abbrev-ref", "HEAD")
	dirty = runGit(root, "status", "--porcelain") != ""

	logOutput := runGit(root, "log", "--oneline", "-10")
	if logOutput != "" {
		commits = strings.Split(logOutput, "\n")
	}

	return
}

func isGitRepo(root string) bool {
	out := runGit(root, "rev-parse", "--is-inside-work-tree")
	return out == "true"
}

func runGit(root string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String())
}
