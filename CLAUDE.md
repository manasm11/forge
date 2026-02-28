# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
Forge is a Go TUI tool that orchestrates automated software development using Claude Code CLI and GitHub CLI. It guides users through a 4-phase workflow: Planning → Review → Inputs → Execution.

## Tech Stack
- Go with charmbracelet/bubbletea for TUI
- State stored in .forge/state.json (no database)
- External CLIs: claude, gh, git

## Architecture
The application follows a phased approach with these core components:

1. **Planning Phase** (`internal/tui/planning.go`) - Interactive conversation with Claude to generate project plans
2. **Review Phase** (`internal/tui/review.go`) - Task management interface for reviewing/editing tasks
3. **Inputs Phase** (`internal/tui/inputs.go`) - Configuration collection for execution settings
4. **Execution Phase** (`internal/executor/`) - Automated task execution with testing and git operations

Key packages:
- `internal/state/` - State management with JSON persistence
- `internal/claude/` - Claude Code CLI wrapper for API interactions
- `internal/executor/` - Task execution engine with git/test integration
- `internal/scanner/` - Project analysis and metadata extraction
- `internal/provider/` - Multi-provider support (Anthropic, Ollama)
- `internal/generator/` - Context file generation for task execution

## Common Development Commands

### Building
```bash
go build .
```

### Testing
```bash
go test ./...
```

### Running a single test
```bash
go test -run TestFunctionName ./path/to/package
```

### Running with race detector
```bash
go run -race .
```

## Code Structure
- `main.go` - Entry point handling preflight checks, state initialization, and TUI startup
- `internal/tui/` - Bubbletea TUI models for each phase
- `internal/state/` - State management and persistence
- `internal/claude/` - Claude Code CLI integration
- `internal/executor/` - Task execution engine
- `internal/scanner/` - Project scanning and metadata extraction
- `internal/preflight/` - Dependency checking
- `internal/provider/` - Multi-provider model support (Anthropic/Ollama)
- `internal/generator/` - Context file generation

## State Management
The application state is stored in `.forge/state.json` with the following key components:
- Tasks with dependency tracking and status lifecycle (pending, in-progress, done, failed, skipped, cancelled)
- Conversation history for context continuity
- Project snapshots for planning context
- Settings for execution configuration
- Plan versioning for replanning support

## Provider Support
The application supports both Anthropic Claude and Ollama providers:
- Configure via `state.Settings.Provider`
- Ollama integration uses reverse proxy pattern with ANTHROPIC_* env vars
- Model selection and validation in `internal/provider/`
- Automatic provider detection at startup with user selection when both are available

## Testing Strategy
- Table-driven tests throughout
- Mock implementations for external dependencies (git, claude, tests)
- Comprehensive test coverage in executor package
- Integration tests for key workflows

## Key Workflows
1. **Project Initialization** - Scan existing codebase and initialize .forge directory
2. **Planning** - Interactive conversation to generate task plans
3. **Replanning** - Update existing plans with context preservation
4. **Task Execution** - Automated code generation, testing, and git operations
5. **State Persistence** - Automatic saving on transitions and exits

## Multi-Provider Support
Forge supports both cloud (Anthropic Claude) and local (Ollama) execution:
- Anthropic: Uses standard Claude CLI with default behavior
- Ollama: Uses reverse proxy pattern or direct 'ollama launch claude' command
- Provider selection happens at startup with automatic detection
- Environment variables configure the connection for each provider type

## Task Execution Engine
The execution engine (`internal/executor/`) orchestrates automated development:
- Reads tasks in dependency order
- For each task: creates branch → executes Claude → runs tests → retries on failure → commits changes
- Comprehensive event system for live dashboard updates
- Real implementations for Git operations, test running, and Claude execution
- Retry logic with context-aware prompts for failed tasks

## TUI Components
The application uses charmbracelet/bubbletea for its terminal interface:
- Modular design with separate models for each phase
- Streaming output support for real-time Claude responses
- Interactive task lists with editing capabilities
- Live execution dashboard with progress tracking