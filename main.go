package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manasm11/forge/internal/claude"
	"github.com/manasm11/forge/internal/executor"
	"github.com/manasm11/forge/internal/preflight"
	"github.com/manasm11/forge/internal/provider"
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

	// 2.5. Check for provider selection (Claude vs Ollama)
	selectedProvider, err := selectProvider(results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting provider: %v\n", err)
		os.Exit(1)
	}

	// 3. Try loading existing forge state
	s, err := state.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	if s == nil {
		// 4a. New forge session — scan the project directory
		snapshot := scanner.Scan(root)

		// Auto-initialize git if not a git repo
		gitResult := scanner.InitGit(root)
		if !gitResult.Initialized {
			fmt.Printf("  Warning: %s\n", gitResult.Error)
		} else if !snapshot.IsExisting {
			// Only show git init message for new projects
			fmt.Println("  Initialized new git repository")
			if gitResult.RemoteURL == "" {
				fmt.Println("  Warning: No remote repository configured")
			}
		}

		// Initialize forge directory and state
		// Create provider configuration based on user selection
		providerCfg := &provider.Config{
			Type:  selectedProvider,
			Model: "sonnet", // Default model, will be overridden by inputs phase
		}
		if selectedProvider == provider.ProviderOllama {
			providerCfg.Model = "qwen3-coder:480b-cloud" // Default Ollama model
			providerCfg.OllamaURL = provider.DefaultOllamaURL()
		}

		s, err = state.InitForgeDir(root, providerCfg, gitResult.Initialized, gitResult.RemoteURL)
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

		// Bug 3 fix: Restore provider from saved state instead of re-detecting
		if s.Settings != nil && s.Settings.Provider.Type != "" {
			selectedProvider = s.Settings.Provider.Type
		}
	}

	// 5. Create Claude client (sonnet model for planning, --max-turns 1 default)
	var claudeClient claude.Claude
	// Use model from state (set during provider init) or fall back to "sonnet"
	model := "sonnet"
	if s.Settings != nil && s.Settings.Provider.Model != "" {
		model = s.Settings.Provider.Model
	}
	// Use saved Ollama URL if available, otherwise default
	ollamaURL := provider.DefaultOllamaURL()
	if s.Settings != nil && s.Settings.Provider.OllamaURL != "" {
		ollamaURL = s.Settings.Provider.OllamaURL
	}
	// Create provider-specific environment variables
	providerEnvVars := provider.EnvVarsForProvider(provider.Config{
		Type:      selectedProvider,
		Model:     model,
		OllamaURL: ollamaURL,
	})

	if c, err := claude.NewClient("claude", 5*time.Minute, model); err != nil {
		// Don't exit — let the TUI start and show error when user tries to chat
		fmt.Printf("  Warning: %v\n", err)
		fmt.Println("  Planning will not work until Claude CLI is available.")
		fmt.Println()
	} else {
		// Set provider-specific environment variables
		claudeClient = c.WithEnvVars(providerEnvVars)
	}

	// 6. Create Claude executor for task execution
	claudeExec := executor.NewRealClaudeExecutor(root)

	// 7. Create app model with state and claude client
	app := tui.NewAppModel(s, root, claudeClient, claudeExec)

	// 7. Run bubbletea
	p := tea.NewProgram(&app, tea.WithAltScreen())

	// Set the program reference for streaming support
	app.SetProgram(p)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}

	// 8. On exit, save final state
	if m, ok := finalModel.(*tui.AppModel); ok {
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

// selectProvider determines which provider to use based on availability and user preference.
func selectProvider(preflightResults []preflight.CheckResult) (provider.ProviderType, error) {
	// Check environment variable first
	envProvider := os.Getenv("FORGE_PROVIDER")
	if envProvider != "" {
		switch strings.ToLower(envProvider) {
		case "claude", "anthropic":
			return provider.ProviderAnthropic, nil
		case "ollama":
			return provider.ProviderOllama, nil
		default:
			fmt.Printf("  Warning: Invalid FORGE_PROVIDER value '%s', ignoring.\n", envProvider)
		}
	}

	// Check which tools are available
	claudeAvailable := false
	ollamaAvailable := false

	for _, r := range preflightResults {
		if r.Name == "claude" && r.Found {
			claudeAvailable = true
		}
	}

	// Check for Ollama availability
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ollamaStatus := provider.DetectOllama(ctx, "")
	if ollamaStatus.Available {
		ollamaAvailable = true
	}

	// Determine provider based on availability
	switch {
	case claudeAvailable && ollamaAvailable:
		// Both available, prompt user for choice
		return promptProviderChoice(ollamaStatus)
	case claudeAvailable:
		// Only Claude available
		fmt.Println("  Using Claude (cloud) provider")
		return provider.ProviderAnthropic, nil
	case ollamaAvailable:
		// Only Ollama available
		fmt.Println("  Using Ollama (local) provider")
		return provider.ProviderOllama, nil
	default:
		// Neither available
		return "", fmt.Errorf("neither Claude CLI nor Ollama is available")
	}
}

// promptProviderChoice asks the user to choose between Claude and Ollama providers
// using an inline bubbletea TUI.
func promptProviderChoice(ollamaStatus provider.OllamaStatus) (provider.ProviderType, error) {
	return tui.RunProviderSelection(ollamaStatus)
}
