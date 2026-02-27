# Forge Development Guidelines

## Project
Forge is a Go TUI tool that orchestrates automated software development using Claude Code CLI and GitHub CLI.

## Tech Stack
- Go with charmbracelet/bubbletea for TUI
- State stored in .forge/state.json (no database)
- External CLIs: claude, gh, git

## Conventions
- Follow standard Go project layout
- All internal packages go in internal/
- Use table-driven tests
- Error messages should be user-friendly (no raw error dumps)
- All state mutations must call state.Save()
- TUI models follow bubbletea patterns: Init(), Update(), View()

## Testing
Run tests: go test ./...

## Structure
- internal/tui/ — bubbletea models for each phase
- internal/state/ — .forge/state.json management
- internal/preflight/ — startup dependency checks
