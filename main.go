package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manasm11/forge/internal/preflight"
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
			fmt.Printf("  ✓ %s (%s)\n", r.Name, r.Version)
		} else {
			fmt.Printf("  ✗ %s — not found: %s\n", r.Name, r.Error)
			allPassed = false
		}
	}

	if !allPassed {
		fmt.Fprintln(os.Stderr, "\nPlease install all required tools before running forge.")
		os.Exit(1)
	}
	fmt.Println("  ✓ All checks passed")
	fmt.Println()

	// 3. Load or initialize state
	s, err := state.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}
	if s == nil {
		s, err = state.Init(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing state: %v\n", err)
			os.Exit(1)
		}
	}

	// 4. Create the app model with loaded state
	appModel := tui.NewAppModel(s, root)

	// 5. Run bubbletea program with altscreen enabled
	p := tea.NewProgram(appModel, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}

	// 6. On exit, save final state
	if m, ok := finalModel.(tui.AppModel); ok {
		if saveErr := state.Save(root, m.State()); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save state on exit: %v\n", saveErr)
		}
	}
}
