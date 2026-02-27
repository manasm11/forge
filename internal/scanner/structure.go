package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Directories to always skip during scanning.
var skipDirs = map[string]bool{
	".git": true, ".forge": true, "node_modules": true, "vendor": true,
	"__pycache__": true, ".venv": true, "venv": true, "dist": true,
	"build": true, "target": true, ".idea": true, ".vscode": true,
	".next": true, ".nuxt": true, "coverage": true,
}

// File extensions counted for LOC estimation.
var codeExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".rs": true, ".java": true, ".rb": true, ".c": true, ".cpp": true, ".h": true,
	".cs": true, ".php": true, ".swift": true, ".kt": true, ".scala": true,
	".html": true, ".css": true, ".sql": true, ".sh": true,
	".yaml": true, ".yml": true, ".json": true, ".toml": true, ".md": true,
}

// Key files to detect in the project.
var keyFileNames = map[string]bool{
	"Dockerfile": true, "docker-compose.yml": true, "docker-compose.yaml": true,
	"Makefile": true, "Justfile": true, "Taskfile.yml": true,
	".gitlab-ci.yml": true, "Jenkinsfile": true,
	"nginx.conf": true, "Caddyfile": true,
	"fly.toml": true, "render.yaml": true, "railway.json": true,
	"vercel.json": true, "netlify.toml": true,
}

const maxLOCFileSize = 1 << 20 // 1MB
const maxTreeDepth = 3
const maxEntriesPerDir = 15
const showEntriesPerDir = 10
const maxTreeLines = 100

// scanStructure walks the directory tree and produces file counts, LOC estimate,
// a tree string (depth 3), and key files found.
func scanStructure(root string) (fileCount int, loc int, structure string, keyFiles []string) {
	type entry struct {
		name  string
		isDir bool
	}

	var buildTree func(dir string, depth int) []string

	buildTree = func(dir string, depth int) []string {
		if depth > maxTreeDepth {
			return nil
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}

		// Separate dirs and files, filter hidden and skipped
		var dirs, files []entry
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") && name != ".github" {
				continue
			}
			if e.IsDir() {
				if skipDirs[name] {
					continue
				}
				dirs = append(dirs, entry{name: name, isDir: true})
			} else {
				files = append(files, entry{name: name, isDir: false})
			}
		}

		// Sort: dirs first, then files, alphabetically
		sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
		sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

		all := append(dirs, files...)
		var lines []string

		shown := len(all)
		truncated := false
		if shown > maxEntriesPerDir {
			shown = showEntriesPerDir
			truncated = true
		}

		indent := strings.Repeat("  ", depth)
		for i := 0; i < shown; i++ {
			e := all[i]
			if e.isDir {
				lines = append(lines, fmt.Sprintf("%s%s/", indent, e.name))
				subLines := buildTree(filepath.Join(dir, e.name), depth+1)
				lines = append(lines, subLines...)
			} else {
				lines = append(lines, fmt.Sprintf("%s%s", indent, e.name))
			}
		}

		if truncated {
			remaining := len(all) - showEntriesPerDir
			lines = append(lines, fmt.Sprintf("%s... and %d more", indent, remaining))
		}

		return lines
	}

	// Walk for file count, LOC, and key files
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		name := d.Name()

		if d.IsDir() {
			if path != root && (skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".github")) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			return nil
		}

		fileCount++

		// Key files detection
		if keyFileNames[name] {
			rel, _ := filepath.Rel(root, path)
			keyFiles = append(keyFiles, filepath.ToSlash(rel))
		}

		// Check for GitHub Actions
		rel, _ := filepath.Rel(root, path)
		relSlash := filepath.ToSlash(rel)
		if strings.HasPrefix(relSlash, ".github/workflows/") && strings.HasSuffix(name, ".yml") {
			keyFiles = append(keyFiles, "GitHub Actions CI found")
		}

		// LOC counting
		ext := strings.ToLower(filepath.Ext(name))
		if !codeExtensions[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > maxLOCFileSize {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			loc++
		}

		return nil
	})

	// Deduplicate key files (GitHub Actions may appear multiple times)
	keyFiles = dedup(keyFiles)

	// Build tree
	treeLines := buildTree(root, 0)
	if len(treeLines) > maxTreeLines {
		treeLines = treeLines[:maxTreeLines]
		treeLines = append(treeLines, "... (tree output truncated)")
	}
	structure = strings.Join(treeLines, "\n")

	return
}

func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
