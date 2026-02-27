//go:build !ci

package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These tests use real git operations in temp directories.

func TestRealGitOps_CreateAndCheckoutBranch(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	err := g.CreateBranch(ctx, "feature/test", "main")
	if err != nil {
		t.Fatalf("CreateBranch error: %v", err)
	}

	branch, err := g.CurrentBranch(ctx)
	if err != nil {
		t.Fatalf("CurrentBranch error: %v", err)
	}
	if branch != "feature/test" {
		t.Errorf("branch = %q, want feature/test", branch)
	}

	err = g.CheckoutBranch(ctx, "main")
	if err != nil {
		t.Fatalf("CheckoutBranch error: %v", err)
	}
}

func TestRealGitOps_StageCommit(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main"), 0644)

	err := g.StageAll(ctx)
	if err != nil {
		t.Fatalf("StageAll error: %v", err)
	}

	has, _ := g.HasStagedChanges(ctx)
	if !has {
		t.Error("should have staged changes")
	}

	sha, err := g.Commit(ctx, "add hello.go")
	if err != nil {
		t.Fatalf("Commit error: %v", err)
	}
	if sha == "" {
		t.Error("SHA should not be empty")
	}
}

func TestRealGitOps_HasUnstagedChanges(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	has, _ := g.HasUnstagedChanges(ctx)
	if has {
		t.Error("clean repo should not have unstaged changes")
	}

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("content"), 0644)

	has, _ = g.HasUnstagedChanges(ctx)
	if !has {
		t.Error("should detect unstaged changes")
	}
}

func TestRealGitOps_BranchExists(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	exists, _ := g.BranchExists(ctx, "main")
	if !exists {
		t.Error("main should exist")
	}

	exists, _ = g.BranchExists(ctx, "nonexistent")
	if exists {
		t.Error("nonexistent should not exist")
	}
}

func TestRealGitOps_ResetHard(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0644)
	g.StageAll(ctx)

	err := g.ResetHard(ctx)
	if err != nil {
		t.Fatalf("ResetHard error: %v", err)
	}

	has, _ := g.HasStagedChanges(ctx)
	if has {
		t.Error("should have no staged changes after reset")
	}
}

func TestRealGitOps_DeleteBranch(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	g.CreateBranch(ctx, "to-delete", "main")
	g.CheckoutBranch(ctx, "main")

	err := g.DeleteBranch(ctx, "to-delete")
	if err != nil {
		t.Fatalf("DeleteBranch error: %v", err)
	}

	exists, _ := g.BranchExists(ctx, "to-delete")
	if exists {
		t.Error("branch should be deleted")
	}
}

func TestRealGitOps_LatestSHA(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	g := NewRealGitOps(dir)
	ctx := context.Background()

	sha, err := g.LatestSHA(ctx)
	if err != nil {
		t.Fatalf("LatestSHA error: %v", err)
	}
	if sha == "" {
		t.Error("SHA should not be empty")
	}
	if len(sha) < 7 {
		t.Errorf("SHA too short: %q", sha)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
