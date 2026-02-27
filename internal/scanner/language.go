package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Known frameworks per language (substring matches in manifest files).
var goFrameworks = []string{"gin", "echo", "fiber", "chi", "gorilla", "bubbletea", "wails"}
var jsFrameworks = []string{"react", "next", "vue", "nuxt", "angular", "express", "fastify", "nestjs", "svelte"}
var pyFrameworks = []string{"django", "flask", "fastapi", "sqlalchemy", "pytorch", "tensorflow"}
var rsFrameworks = []string{"actix", "axum", "tokio", "rocket", "serde"}
var dartFrameworks = []string{"flutter", "riverpod", "bloc", "dio"}

// detectLanguage examines manifest files to determine the primary language,
// frameworks, and dependencies.
func detectLanguage(root string) (language string, frameworks []string, dependencies []string) {
	// Detection priority: first match wins for primary language
	type detector struct {
		file     string
		language string
		detect   func(path string) (string, []string, []string)
	}

	detectors := []detector{
		{"go.mod", "Go", detectGo},
		{"package.json", "", detectJS}, // language determined by tsconfig presence
		{"requirements.txt", "Python", detectPythonReqs},
		{"pyproject.toml", "Python", detectPythonPyproject},
		{"setup.py", "Python", nil},
		{"Pipfile", "Python", nil},
		{"Cargo.toml", "Rust", detectRust},
		{"pom.xml", "Java", nil},
		{"build.gradle", "Java", nil},
		{"build.gradle.kts", "Kotlin", nil},
		{"Gemfile", "Ruby", nil},
		{"composer.json", "PHP", nil},
		{"Package.swift", "Swift", nil},
		{"pubspec.yaml", "Dart/Flutter", detectDart},
		{"mix.exs", "Elixir", nil},
	}

	for _, d := range detectors {
		path := filepath.Join(root, d.file)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		if d.detect != nil {
			lang, fw, deps := d.detect(path)
			if lang == "" {
				lang = d.language
			}
			return lang, fw, deps
		}

		return d.language, nil, nil
	}

	return "", nil, nil
}

func detectGo(path string) (string, []string, []string) {
	lines := readLines(path, 200)
	var deps []string
	var frameworks []string
	inRequire := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "require (" {
			inRequire = true
			continue
		}
		if trimmed == ")" {
			inRequire = false
			continue
		}

		if inRequire && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 1 {
				dep := parts[0]
				deps = append(deps, dep)

				// Check for known frameworks
				lower := strings.ToLower(dep)
				for _, fw := range goFrameworks {
					if strings.Contains(lower, fw) {
						frameworks = append(frameworks, fw)
					}
				}
			}
		}
	}

	if len(deps) > 20 {
		deps = deps[:20]
	}
	return "Go", frameworks, deps
}

func detectJS(path string) (string, []string, []string) {
	language := "JavaScript"
	dir := filepath.Dir(path)
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		language = "TypeScript"
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return language, nil, nil
	}

	var deps []string
	var frameworks []string

	// Simple string matching for dependencies
	lines := strings.Split(string(content), "\n")
	inDeps := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, `"dependencies"`) || strings.Contains(trimmed, `"devDependencies"`) {
			inDeps = true
			continue
		}
		if inDeps && trimmed == "}" {
			inDeps = false
			continue
		}

		if inDeps && strings.Contains(trimmed, `"`) {
			// Extract package name from "name": "version"
			parts := strings.SplitN(trimmed, `"`, 4)
			if len(parts) >= 2 {
				depName := parts[1]
				if depName != "" && !strings.HasPrefix(depName, "@types/") {
					deps = append(deps, depName)

					lower := strings.ToLower(depName)
					for _, fw := range jsFrameworks {
						if lower == fw || strings.HasPrefix(lower, fw+"/") || strings.HasSuffix(lower, "/"+fw) {
							frameworks = append(frameworks, fw)
						}
					}
				}
			}
		}
	}

	if len(deps) > 20 {
		deps = deps[:20]
	}
	return language, dedup(frameworks), deps
}

func detectPythonReqs(path string) (string, []string, []string) {
	lines := readLines(path, 200)
	var deps []string
	var frameworks []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Extract package name (before ==, >=, ~=, etc.)
		name := trimmed
		for _, sep := range []string{"==", ">=", "<=", "~=", "!=", ">", "<", "["} {
			if idx := strings.Index(name, sep); idx > 0 {
				name = name[:idx]
			}
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		deps = append(deps, name)

		lower := strings.ToLower(name)
		for _, fw := range pyFrameworks {
			if lower == fw || strings.HasPrefix(lower, fw+"-") {
				frameworks = append(frameworks, fw)
			}
		}
	}

	if len(deps) > 20 {
		deps = deps[:20]
	}
	return "Python", frameworks, deps
}

func detectPythonPyproject(path string) (string, []string, []string) {
	lines := readLines(path, 300)
	var deps []string
	var frameworks []string
	inDeps := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "dependencies") && strings.Contains(trimmed, "[") {
			inDeps = true
			continue
		}
		if inDeps && trimmed == "]" {
			inDeps = false
			continue
		}

		if inDeps && strings.Contains(trimmed, `"`) {
			// Extract name from quoted dependency string
			name := extractDepName(trimmed)
			if name != "" {
				deps = append(deps, name)

				lower := strings.ToLower(name)
				for _, fw := range pyFrameworks {
					if lower == fw || strings.HasPrefix(lower, fw+"-") {
						frameworks = append(frameworks, fw)
					}
				}
			}
		}
	}

	if len(deps) > 20 {
		deps = deps[:20]
	}
	return "Python", frameworks, deps
}

func detectRust(path string) (string, []string, []string) {
	lines := readLines(path, 200)
	var deps []string
	var frameworks []string
	inDeps := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[dependencies]" || trimmed == "[dev-dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && inDeps {
			inDeps = false
			continue
		}

		if inDeps && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			name := strings.TrimSpace(parts[0])
			if name != "" {
				deps = append(deps, name)

				lower := strings.ToLower(name)
				for _, fw := range rsFrameworks {
					if strings.Contains(lower, fw) {
						frameworks = append(frameworks, fw)
					}
				}
			}
		}
	}

	if len(deps) > 20 {
		deps = deps[:20]
	}
	return "Rust", frameworks, deps
}

func detectDart(path string) (string, []string, []string) {
	lines := readLines(path, 200)
	var deps []string
	var frameworks []string
	inDeps := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "dependencies:" || trimmed == "dev_dependencies:" {
			inDeps = true
			continue
		}
		// New top-level key ends the dependencies section
		if inDeps && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trimmed, ":") {
			inDeps = false
		}

		if inDeps && strings.Contains(trimmed, ":") {
			name := strings.SplitN(trimmed, ":", 2)[0]
			name = strings.TrimSpace(name)
			if name != "" && name != "sdk" {
				deps = append(deps, name)

				lower := strings.ToLower(name)
				for _, fw := range dartFrameworks {
					if lower == fw {
						frameworks = append(frameworks, fw)
					}
				}
			}
		}
	}

	if len(deps) > 20 {
		deps = deps[:20]
	}
	return "Dart/Flutter", frameworks, deps
}

// readLines reads up to maxLines from a file.
func readLines(path string, maxLines int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() && len(lines) < maxLines {
		lines = append(lines, s.Text())
	}
	return lines
}

// extractDepName extracts a package name from a quoted dependency string like `"django>=3.0"`.
func extractDepName(s string) string {
	// Find content between quotes
	start := strings.IndexByte(s, '"')
	if start == -1 {
		return ""
	}
	end := strings.IndexByte(s[start+1:], '"')
	if end == -1 {
		return ""
	}
	dep := s[start+1 : start+1+end]

	// Strip version specifiers
	for _, sep := range []string{">=", "<=", "==", "~=", "!=", ">", "<", "[", " "} {
		if idx := strings.Index(dep, sep); idx > 0 {
			dep = dep[:idx]
		}
	}
	return strings.TrimSpace(dep)
}
