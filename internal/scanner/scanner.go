package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/manasm11/forge/internal/state"
)

// Scan analyzes the project directory and returns a snapshot.
// It never fails — if any individual scan step errors, it
// just leaves that field empty and continues.
func Scan(root string) state.ProjectSnapshot {
	snap := state.ProjectSnapshot{}

	// Check if directory has any code files (not just .forge/)
	if !hasCodeFiles(root) {
		snap.IsExisting = false
		return snap
	}

	// Scan structure
	snap.FileCount, snap.LOC, snap.Structure, snap.KeyFiles = scanStructure(root)

	// Detect language and frameworks
	snap.Language, snap.Frameworks, snap.Dependencies = detectLanguage(root)

	// Scan git info
	snap.GitBranch, snap.GitDirty, snap.RecentCommits = scanGit(root)

	// Read README
	snap.ReadmeContent = readFileHead(root, "README.md", 200)
	if snap.ReadmeContent == "" {
		snap.ReadmeContent = readFileHead(root, "README", 200)
	}

	// Read CLAUDE.md
	snap.ClaudeMD = readFileFull(root, "CLAUDE.md")

	snap.IsExisting = true
	return snap
}

// hasCodeFiles checks whether the directory contains any files besides .forge/.
func hasCodeFiles(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if name == ".forge" || name == ".git" {
			continue
		}
		// Any other file or directory means there's project content
		return true
	}
	return false
}

// readFileHead reads up to maxLines lines from a file relative to root.
func readFileHead(root, name string, maxLines int) string {
	lines := readLines(filepath.Join(root, name), maxLines)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// readFileFull reads the entire file content relative to root (up to 1MB).
func readFileFull(root, name string) string {
	path := filepath.Join(root, name)
	info, err := os.Stat(path)
	if err != nil || info.Size() > 1<<20 {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
