package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/preflight"
	"github.com/manasm11/forge/internal/scanner"
	"github.com/manasm11/forge/internal/state"
	"github.com/manasm11/forge/internal/tui"
)

func main() {
	// 1. Determine project root (current working directory)
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine working directory: %v\n", err)
		os.Exit(1)
	}

	// 2. Run preflight checks
	results := preflight.RunAll()
	allPassed := true
	for _, r := range results {
		if r.Found {
			fmt.Printf("  \u2713 %s (%s)\n", r.Name, r.Version)
		} else {
			fmt.Printf("  \u2717 %s \u2014 not found: %s\n", r.Name, r.Error)
			allPassed = false
		}
	}

	if !allPassed {
		fmt.Fprintln(os.Stderr, "\nPlease install all required tools before running forge.")
		os.Exit(1)
	}
	fmt.Println("  \u2713 All checks passed")
	fmt.Println()

	// 3. Try loading existing forge state
	s, err := state.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	if s == nil {
		// 4a. New forge session — scan the project directory
		snapshot := scanner.Scan(root)

		// Initialize forge directory and state
		s, err = state.InitForgeDir(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing state: %v\n", err)
			os.Exit(1)
		}
		s.Snapshot = &snapshot

		if snapshot.IsExisting {
			detail := snapshot.Language
			if detail == "" {
				detail = "unknown language"
			}
			fwInfo := ""
			if len(snapshot.Frameworks) > 0 {
				fwInfo = ", " + joinFrameworks(snapshot.Frameworks)
			}
			fmt.Printf("  Detected existing %s project (%d files%s)\n", detail, snapshot.FileCount, fwInfo)
		} else {
			fmt.Println("  No existing project detected")
		}

		if err := state.Save(root, s); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
		}

		fmt.Println()
		fmt.Println("  Created .forge/ directory")
		fmt.Println("  \u2514\u2500\u2500 .forge/logs/ will not be committed (has its own .gitignore)")
		fmt.Println("  \u2514\u2500\u2500 .forge/state.json tracks your plan and progress")
		fmt.Println("  Tip: Commit .forge/ to share project plans with your team.")
		fmt.Println("       Or add .forge/ to .gitignore to keep plans local.")
		fmt.Println()
	} else {
		// 4b. Resuming existing forge session
		completed := len(s.CompletedTasks())
		total := len(s.Tasks)
		fmt.Printf("  Resuming forge session (Phase: %s, %d/%d tasks done)\n\n", s.Phase, completed, total)
	}

	// 5. Create Claude client
	claudeClient, err := claude.NewClient("claude", 5*time.Minute)
	if err != nil {
		// Don't exit — let the TUI start and show error when user tries to chat
		fmt.Printf("  Warning: %v\n", err)
		fmt.Println("  Planning will not work until Claude CLI is available.")
		fmt.Println()
	}

	// 6. Create app model with state and claude client
	app := tui.NewAppModel(s, root, claudeClient)

	// 7. Run bubbletea
	p := tea.NewProgram(app, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}

	// 8. On exit, save final state
	if m, ok := finalModel.(tui.AppModel); ok {
		if saveErr := state.Save(root, m.State()); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save state on exit: %v\n", saveErr)
		}
	}
}

func joinFrameworks(frameworks []string) string {
	if len(frameworks) == 0 {
		return ""
	}
	result := frameworks[0]
	for i := 1; i < len(frameworks); i++ {
		result += " + " + frameworks[i]
	}
	return result
}
