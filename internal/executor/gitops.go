package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// RealGitOps implements GitOps using real git commands.
type RealGitOps struct {
	dir string
}

// NewRealGitOps creates a GitOps implementation rooted at dir.
func NewRealGitOps(dir string) *RealGitOps {
	return &RealGitOps{dir: dir}
}

func (g *RealGitOps) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("git %s: %s: %w", args[0], output, err)
	}
	return output, nil
}

func (g *RealGitOps) CurrentBranch(ctx context.Context) (string, error) {
	return g.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

func (g *RealGitOps) CreateBranch(ctx context.Context, name, baseBranch string) error {
	if _, err := g.run(ctx, "checkout", "-b", name, baseBranch); err != nil {
		return err
	}
	return nil
}

func (g *RealGitOps) CheckoutBranch(ctx context.Context, name string) error {
	_, err := g.run(ctx, "checkout", name)
	return err
}

func (g *RealGitOps) BranchExists(ctx context.Context, name string) (bool, error) {
	_, err := g.run(ctx, "rev-parse", "--verify", "refs/heads/"+name)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (g *RealGitOps) StageAll(ctx context.Context) error {
	_, err := g.run(ctx, "add", "-A")
	return err
}

func (g *RealGitOps) HasStagedChanges(ctx context.Context) (bool, bool, error) {
	out, err := g.run(ctx, "diff", "--cached", "--name-only")
	if err != nil {
		return false, false, err
	}
	hasStaged := out != ""
	hasUnstaged, _ := g.HasUnstagedChanges(ctx)
	return hasStaged, hasUnstaged, nil
}

func (g *RealGitOps) HasUnstagedChanges(ctx context.Context) (bool, error) {
	out, err := g.run(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (g *RealGitOps) Commit(ctx context.Context, message string) (string, error) {
	if _, err := g.run(ctx, "commit", "-m", message); err != nil {
		return "", err
	}
	sha, err := g.run(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return sha, nil
}

func (g *RealGitOps) Push(ctx context.Context) error {
	_, err := g.run(ctx, "push", "-u", "origin", "HEAD")
	return err
}

func (g *RealGitOps) Merge(ctx context.Context, branch string) error {
	_, err := g.run(ctx, "merge", "--no-ff", branch)
	return err
}

func (g *RealGitOps) LatestSHA(ctx context.Context) (string, error) {
	return g.run(ctx, "rev-parse", "HEAD")
}

func (g *RealGitOps) ResetHard(ctx context.Context) error {
	_, err := g.run(ctx, "reset", "--hard", "HEAD")
	return err
}

func (g *RealGitOps) DeleteBranch(ctx context.Context, name string) error {
	_, err := g.run(ctx, "branch", "-D", name)
	return err
}
