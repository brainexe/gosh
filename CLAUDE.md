# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

- **Build**: `make build` - Compiles the gosh binary to `build/gosh`
- **Test**: `make test` - Runs unit tests with `go test -v ./...`
- **Test with race detection**: `make test-race` - Runs tests with race detection enabled
- **Lint**: `make lint` - Runs golangci-lint with auto-fix enabled
- **Integration tests**: `make test-integration` - Builds and runs integration tests via `test_integration.sh`
- **Clean**: `make clean` - Removes build artifacts
- **Full build**: `make all` - Runs test, lint, and build in sequence

Always run `make lint` after making code changes to ensure code quality.

## Project Architecture

### Core Functionality
`gosh` is a Go-based SSH session manager that executes commands across multiple hosts in parallel. The entire application is contained in `main.go` with supporting tests in `main_test.go`.

### Key Components

**Command Execution Flow:**
- `main()` - Parses CLI flags using spf13/pflag library
- `executeCommand()` - Orchestrates parallel SSH execution using goroutines and sync.WaitGroup
- `runSSH()` - Handles individual SSH connections using `ssh` command with timeout and batch mode
- `formatHost()` - Creates colored output prefixes for host identification

**Interactive Mode:**
- `interactiveMode()` - Provides a REPL-like interface for running commands
- Built-in commands: `help`, `exit`/`quit`
- All other input is treated as SSH commands to execute on all hosts

**Output Management:**
- Uses ANSI color codes for host differentiation (16 predefined colors)
- Supports `--no-color` flag to disable colored output
- Each host's output is prefixed with colored/formatted hostname

### SSH Configuration
- Uses system `ssh` command with connection timeout of 5 seconds
- Enables batch mode (`BatchMode=yes`) for non-interactive operation
- Supports custom usernames via `-u/--user` flag
- All SSH connections run in parallel using goroutines

### Testing Strategy
- Unit tests in `main_test.go` for utility functions (`maxLen`, `formatHost`)
- Integration tests in `test_integration.sh` require `.env` file with `TEST_HOSTS` configuration
- Integration tests validate real SSH connections and command execution

### Environment Configuration
Integration tests require a `.env` file (copy from `.env.example`) with:
- `TEST_HOSTS` - Space-separated list of hostnames/IPs for testing
- `TEST_SSH_USER` - Optional SSH username for tests
- `TEST_SSH_TIMEOUT` - SSH timeout in seconds (default: 5)

### CI/CD
- GitHub Actions workflow tests on Go 1.24.x and 1.25.x
- Cross-platform builds for Linux, macOS, and Windows (amd64)
- Comprehensive golangci-lint configuration with 40+ enabled linters