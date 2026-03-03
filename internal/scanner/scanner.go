package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectSnapshot holds detected project context for the planning phase.
type ProjectSnapshot struct {
	IsExisting    bool     `json:"is_existing"`
	Language      string   `json:"language,omitempty"`
	Frameworks    []string `json:"frameworks,omitempty"`
	Dependencies  []string `json:"dependencies,omitempty"`
	FileCount     int      `json:"file_count"`
	LOC           int      `json:"loc_estimate"`
	Structure     string   `json:"structure"`
	ReadmeContent string   `json:"readme,omitempty"`
	ClaudeMD      string   `json:"claude_md,omitempty"`
	GitBranch     string   `json:"git_branch,omitempty"`
	GitDirty      bool     `json:"git_dirty"`
	RecentCommits []string `json:"recent_commits,omitempty"`
	KeyFiles      []string `json:"key_files,omitempty"`
}

// Scan analyzes the project directory and returns a snapshot.
// It never fails — if any individual scan step errors, it
// just leaves that field empty and continues.
func Scan(root string) ProjectSnapshot {
	snap := ProjectSnapshot{}

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
