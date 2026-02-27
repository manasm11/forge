package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanStructure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a known file structure
	dirs := []string{
		"cmd/server",
		"internal/handlers",
		"internal/models",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"go.mod":                    "module test\n\ngo 1.21\n",
		"README.md":                 "# Test Project\n",
		"cmd/server/main.go":        "package main\nfunc main() {}\n",
		"internal/handlers/user.go": "package handlers\n\nfunc GetUser() {}\nfunc CreateUser() {}\n",
		"internal/models/user.go":   "package models\n\ntype User struct{}\n",
		"Dockerfile":                "FROM golang:1.21\n",
		"Makefile":                  "build:\n\tgo build ./...\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	fileCount, loc, structure, keyFiles := scanStructure(root)

	// File count should match (7 files, not counting directories)
	if fileCount != 7 {
		t.Errorf("fileCount = %d, want 7", fileCount)
	}

	// LOC should be positive
	if loc <= 0 {
		t.Errorf("loc = %d, want > 0", loc)
	}

	// Structure should contain directory names
	if !strings.Contains(structure, "cmd/") {
		t.Errorf("structure should contain 'cmd/', got:\n%s", structure)
	}
	if !strings.Contains(structure, "internal/") {
		t.Errorf("structure should contain 'internal/', got:\n%s", structure)
	}

	// Key files should include Dockerfile and Makefile
	foundDockerfile := false
	foundMakefile := false
	for _, kf := range keyFiles {
		if kf == "Dockerfile" {
			foundDockerfile = true
		}
		if kf == "Makefile" {
			foundMakefile = true
		}
	}
	if !foundDockerfile {
		t.Errorf("keyFiles should include Dockerfile, got %v", keyFiles)
	}
	if !foundMakefile {
		t.Errorf("keyFiles should include Makefile, got %v", keyFiles)
	}
}

func TestScanStructureSkipsDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a node_modules dir (should be skipped)
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "lodash"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "lodash", "index.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a real source file
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("console.log('hello')"), 0644); err != nil {
		t.Fatal(err)
	}

	fileCount, _, structure, _ := scanStructure(root)

	if fileCount != 1 {
		t.Errorf("fileCount = %d, want 1 (node_modules should be skipped)", fileCount)
	}
	if strings.Contains(structure, "node_modules") {
		t.Errorf("structure should not contain node_modules:\n%s", structure)
	}
}

func TestDetectLanguageGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	gomod := `module example.com/test

go 1.21

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/lib/pq v1.10.0
	github.com/stretchr/testify v1.8.0
)
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}

	lang, frameworks, deps := detectLanguage(root)

	if lang != "Go" {
		t.Errorf("language = %q, want %q", lang, "Go")
	}

	foundGin := false
	for _, fw := range frameworks {
		if fw == "gin" {
			foundGin = true
		}
	}
	if !foundGin {
		t.Errorf("frameworks should contain 'gin', got %v", frameworks)
	}

	if len(deps) != 3 {
		t.Errorf("deps length = %d, want 3", len(deps))
	}
}

func TestDetectLanguageJS(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	packageJSON := `{
  "name": "test-project",
  "dependencies": {
    "react": "^18.0.0",
    "next": "^14.0.0",
    "axios": "^1.0.0"
  }
}
`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	lang, frameworks, deps := detectLanguage(root)

	if lang != "JavaScript" {
		t.Errorf("language = %q, want %q", lang, "JavaScript")
	}

	if len(frameworks) < 2 {
		t.Errorf("expected at least 2 frameworks (react, next), got %v", frameworks)
	}

	if len(deps) == 0 {
		t.Error("expected non-empty deps")
	}
}

func TestDetectLanguageTS(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	lang, _, _ := detectLanguage(root)

	if lang != "TypeScript" {
		t.Errorf("language = %q, want %q", lang, "TypeScript")
	}
}

func TestDetectLanguageEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	lang, frameworks, deps := detectLanguage(root)

	if lang != "" {
		t.Errorf("language = %q, want empty", lang)
	}
	if len(frameworks) != 0 {
		t.Errorf("frameworks = %v, want empty", frameworks)
	}
	if len(deps) != 0 {
		t.Errorf("deps = %v, want empty", deps)
	}
}

func TestDetectLanguagePython(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	requirements := `django>=4.0
flask==2.0.0
requests
numpy>=1.21
`
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatal(err)
	}

	lang, frameworks, deps := detectLanguage(root)

	if lang != "Python" {
		t.Errorf("language = %q, want %q", lang, "Python")
	}

	if len(deps) != 4 {
		t.Errorf("deps length = %d, want 4", len(deps))
	}

	foundDjango := false
	foundFlask := false
	for _, fw := range frameworks {
		if fw == "django" {
			foundDjango = true
		}
		if fw == "flask" {
			foundFlask = true
		}
	}
	if !foundDjango {
		t.Errorf("frameworks should contain 'django', got %v", frameworks)
	}
	if !foundFlask {
		t.Errorf("frameworks should contain 'flask', got %v", frameworks)
	}
}

func TestDetectLanguageRust(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	cargoToml := `[package]
name = "test"
version = "0.1.0"

[dependencies]
actix-web = "4"
serde = { version = "1.0", features = ["derive"] }
tokio = { version = "1", features = ["full"] }
`
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		t.Fatal(err)
	}

	lang, frameworks, deps := detectLanguage(root)

	if lang != "Rust" {
		t.Errorf("language = %q, want %q", lang, "Rust")
	}

	if len(deps) != 3 {
		t.Errorf("deps length = %d, want 3", len(deps))
	}

	foundActix := false
	for _, fw := range frameworks {
		if fw == "actix" {
			foundActix = true
		}
	}
	if !foundActix {
		t.Errorf("frameworks should contain 'actix', got %v", frameworks)
	}
}

func TestScanGitNonRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	branch, dirty, commits := scanGit(root)

	if branch != "" {
		t.Errorf("branch = %q, want empty for non-git dir", branch)
	}
	if dirty {
		t.Error("dirty should be false for non-git dir")
	}
	if len(commits) != 0 {
		t.Errorf("commits = %v, want empty for non-git dir", commits)
	}
}

func TestScanGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()

	// Initialize a git repo with a commit
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", c, err, out)
		}
	}

	// Create a file and commit
	if err := os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	commitCmds := [][]string{
		{"git", "add", "test.txt"},
		{"git", "commit", "-m", "initial commit"},
	}
	for _, c := range commitCmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", c, err, out)
		}
	}

	branch, dirty, commits := scanGit(root)

	// Branch should be set (either "main" or "master" depending on git config)
	if branch == "" {
		t.Error("branch should not be empty for git repo with commits")
	}

	// Should not be dirty (all changes committed)
	if dirty {
		t.Error("dirty should be false after committing all changes")
	}

	// Should have at least one commit
	if len(commits) == 0 {
		t.Error("commits should have at least one entry")
	}
	if !strings.Contains(commits[0], "initial commit") {
		t.Errorf("first commit should contain 'initial commit', got %q", commits[0])
	}

	// Make it dirty
	if err := os.WriteFile(filepath.Join(root, "test2.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	_, dirty2, _ := scanGit(root)
	if !dirty2 {
		t.Error("dirty should be true after adding untracked file")
	}
}

func TestScanFullIntegration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a minimal Go project
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Test\n\nA test project.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	snap := Scan(root)

	if !snap.IsExisting {
		t.Error("IsExisting should be true for directory with code files")
	}
	if snap.Language != "Go" {
		t.Errorf("Language = %q, want %q", snap.Language, "Go")
	}
	if snap.FileCount < 3 {
		t.Errorf("FileCount = %d, want >= 3", snap.FileCount)
	}
	if snap.ReadmeContent == "" {
		t.Error("ReadmeContent should not be empty")
	}
	if !strings.Contains(snap.ReadmeContent, "A test project") {
		t.Errorf("ReadmeContent should contain readme text, got %q", snap.ReadmeContent)
	}
}

func TestScanEmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	snap := Scan(root)

	if snap.IsExisting {
		t.Error("IsExisting should be false for empty directory")
	}
}

func TestScanDirWithOnlyForge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Only .forge/ and .git/ directories
	if err := os.MkdirAll(filepath.Join(root, ".forge"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	snap := Scan(root)

	if snap.IsExisting {
		t.Error("IsExisting should be false when only .forge/ and .git/ exist")
	}
}

func TestGitHubActionsDetection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create GitHub Actions workflow
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte("name: CI\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Also need a non-hidden file so fileCount is non-zero
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, _, keyFiles := scanStructure(root)

	found := false
	for _, kf := range keyFiles {
		if kf == "GitHub Actions CI found" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("keyFiles should contain 'GitHub Actions CI found', got %v", keyFiles)
	}
}

func TestScan_GoProjectWithFrameworks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTestFile(t, dir, "go.mod", `module example.com/test

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	gorm.io/gorm v1.25.0
)`)
	writeTestFile(t, dir, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}`)
	writeTestFile(t, dir, "README.md", "# Test Project\n\nThis is a test.")
	os.MkdirAll(filepath.Join(dir, "internal", "handlers"), 0755)
	writeTestFile(t, dir, "internal/handlers/user.go", `package handlers`)

	snap := Scan(dir)

	if !snap.IsExisting {
		t.Error("should detect existing project")
	}
	if snap.Language != "Go" {
		t.Errorf("Language = %q, want Go", snap.Language)
	}
	if !containsStr(snap.Frameworks, "gin") {
		t.Errorf("Frameworks = %v, should contain gin", snap.Frameworks)
	}
	if snap.FileCount < 3 {
		t.Errorf("FileCount = %d, want >= 3", snap.FileCount)
	}
	if snap.ReadmeContent == "" {
		t.Error("ReadmeContent should not be empty")
	}
	if !strings.Contains(snap.Structure, "internal") {
		t.Error("Structure should contain 'internal' directory")
	}
}

func TestScan_NodeProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTestFile(t, dir, "package.json", `{
  "name": "test-app",
  "dependencies": {
    "react": "^18.0.0",
    "next": "^14.0.0",
    "axios": "^1.0.0"
  }
}`)
	writeTestFile(t, dir, "tsconfig.json", `{"compilerOptions": {}}`)
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	writeTestFile(t, dir, "src/index.tsx", `export default function App() {}`)

	snap := Scan(dir)

	if !snap.IsExisting {
		t.Error("should detect existing project")
	}
	if snap.Language != "TypeScript" {
		t.Errorf("Language = %q, want TypeScript", snap.Language)
	}
	if !containsStr(snap.Frameworks, "next") || !containsStr(snap.Frameworks, "react") {
		t.Errorf("Frameworks = %v, should contain next and react", snap.Frameworks)
	}
}

func TestScan_PythonProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTestFile(t, dir, "requirements.txt", "django==4.2\ncelery==5.3\nredis==5.0\n")
	writeTestFile(t, dir, "manage.py", `#!/usr/bin/env python`)

	snap := Scan(dir)

	if snap.Language != "Python" {
		t.Errorf("Language = %q, want Python", snap.Language)
	}
	if !containsStr(snap.Frameworks, "django") {
		t.Errorf("Frameworks = %v, should contain django", snap.Frameworks)
	}
}

func TestScan_WithClaudeMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTestFile(t, dir, "main.go", "package main")
	writeTestFile(t, dir, "go.mod", "module test")
	claudeContent := "# Project Guidelines\n\nUse Go conventions."
	writeTestFile(t, dir, "CLAUDE.md", claudeContent)

	snap := Scan(dir)

	if snap.ClaudeMD != claudeContent {
		t.Errorf("ClaudeMD = %q, want %q", snap.ClaudeMD, claudeContent)
	}
}

func TestScan_SkipsIgnoredDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeTestFile(t, dir, "main.go", "package main")
	writeTestFile(t, dir, "go.mod", "module test")
	os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
	writeTestFile(t, dir, "node_modules/pkg/index.js", "module.exports = {}")
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0755)
	writeTestFile(t, dir, ".git/objects/test", "binary")
	os.MkdirAll(filepath.Join(dir, "vendor", "lib"), 0755)
	writeTestFile(t, dir, "vendor/lib/dep.go", "package lib")

	snap := Scan(dir)

	if strings.Contains(snap.Structure, "node_modules") {
		t.Error("Structure should not contain node_modules")
	}
	if strings.Contains(snap.Structure, ".git") {
		t.Error("Structure should not contain .git")
	}
}

func TestScanGit_WithCommits(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()

	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "test@test.com")
	runTestGit(t, dir, "config", "user.name", "Test")

	writeTestFile(t, dir, "file.txt", "hello")
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "initial commit")

	writeTestFile(t, dir, "file2.txt", "world")
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "add file2")

	branch, dirty, commits := scanGit(dir)

	if branch == "" {
		t.Error("branch should not be empty")
	}
	if dirty {
		t.Error("should not be dirty after commit")
	}
	if len(commits) != 2 {
		t.Errorf("commits count = %d, want 2", len(commits))
	}
}

func TestScanGit_DirtyWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()

	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "test@test.com")
	runTestGit(t, dir, "config", "user.name", "Test")

	writeTestFile(t, dir, "file.txt", "hello")
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "initial")

	// Make dirty
	writeTestFile(t, dir, "uncommitted.txt", "dirty")

	_, dirty, _ := scanGit(dir)

	if !dirty {
		t.Error("should be dirty with uncommitted files")
	}
}

// Helper functions

func writeTestFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	os.MkdirAll(filepath.Dir(full), 0755)
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
