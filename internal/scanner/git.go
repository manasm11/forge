package scanner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

const gitTimeout = 5 * time.Second

// GitInitResult contains the result of git initialization.
type GitInitResult struct {
	Initialized bool
	Branch      string
	RemoteURL   string
	Error       string
}

// InitGit initializes a git repository if one doesn't exist.
// Returns GitInitResult with initialization details.
func InitGit(root string) GitInitResult {
	result := GitInitResult{}

	// Check if already a git repo
	if isGitRepo(root) {
		result.Initialized = true
		result.Branch = getCurrentBranch(root)
		result.RemoteURL = getRemoteURL(root)
		return result
	}

	// Initialize git repository
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = root

	if err := cmd.Run(); err != nil {
		result.Error = "failed to initialize git: " + err.Error()
		return result
	}

	result.Initialized = true
	result.Branch = getCurrentBranch(root)
	result.RemoteURL = "" // No remote after init

	return result
}

// DetectBaseBranch detects the main branch from common names.
func DetectBaseBranch(root string) string {
	branches := []string{"main", "master", "develop", "dev", "trunk"}

	for _, branch := range branches {
		if branchExists(root, branch) {
			return branch
		}
	}

	// Default to main if no common branch found
	return "main"
}

// branchExists checks if a branch exists locally.
func branchExists(root, branch string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", branch)
	cmd.Dir = root

	return cmd.Run() == nil
}

// getRemoteURL returns the origin remote URL if set.
func getRemoteURL(root string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = root

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String())
}

// getCurrentBranch returns the current branch name.
func getCurrentBranch(root string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "main"
	}

	return strings.TrimSpace(stdout.String())
}

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

// AddRemote adds a remote to the git repository.
func AddRemote(root, name, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "remote", "add", name, url)
	cmd.Dir = root

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// GitInitialized returns true if the directory has a .git folder.
func GitInitialized(root string) bool {
	_, err := os.Stat(root + "/.git")
	return err == nil
}
