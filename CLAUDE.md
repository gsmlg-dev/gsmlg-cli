# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `gsmlg-cli`, a personal command-line tool written in Go using the Cobra framework. It provides various utilities for managing blog posts, DNS records, RSA keys, HTTP benchmarking, OPNsense services, and semantic releases.

## Key Architecture

### Command Structure
- **Entry point**: `main.go` calls `cmd.Execute()`
- **Root command**: `cmd/root.go` defines the base command and configuration handling
- **Subcommands**: Each subcommand is defined in separate files under `cmd/`:
  - `blog.go` - Blog management (fetches from remote API)
  - `zdns.go` - ZDNS cloud DNS management with authentication
  - `rsa.go` - RSA key pair management
  - `opnsense.go` - OPNsense service management
  - `httpbenchmark.go` - HTTP benchmarking tool
  - `semanticRelease.go` - Semantic release automation
  - `replace.go` - Text replacement utility
  - Additional zone/record management commands

### Configuration
- Uses Viper for configuration management
- Default config location: `$HOME/.config/gsmlg/cli.yaml`
- Config file stores persistent data like ZDNS authentication tokens
- Override config location with `--config` flag

### Dependencies
- Uses `github.com/gsmlg-dev/gsmlg-golang` as a core library (can be replaced locally for development)
- Built with `spf13/cobra` for CLI framework and `spf13/viper` for configuration
- Semantic release functionality uses `go-semantic-release/semantic-release/v2`

## Development Commands

### Building
```bash
# Standard build
make build

# Cross-platform builds (via semantic-release)
# See .releaserc.yaml for all supported platforms:
# - linux/amd64, linux/arm64
# - darwin/amd64, darwin/arm64
# - windows/amd64, windows/arm64
# - freebsd/amd64

# Build with version injection
go build -ldflags "-w -s -X github.com/gsmlg-dev/gsmlg-cli/cmd.Version=x.y.z" -o gsmlg-cli main.go
```

### Development Setup
```bash
# Setup for local development (uses local gsmlg-golang)
make setup-dev

# Setup for CI (clones gsmlg-golang)
make setup-ci

# Clean build artifacts
make clean
```

### Running Commands
```bash
# Get help for any command
./gsmlg-cli help
./gsmlg-cli [command] --help

# Example: ZDNS login
./gsmlg-cli zdns --username myuser

# Example: List blogs
./gsmlg-cli blog --output json
```

## Important Implementation Details

### Version Management
- Version is set via `-ldflags` during build: `-X github.com/gsmlg-dev/gsmlg-cli/cmd.Version=x.y.z`
- Default version is "dev" (see `cmd/root.go:34`)
- Print version: `gsmlg-cli -v`

### Error Handling
- Uses custom `exitIfError` handler from `gsmlg-golang/errorhandler`
- Defined in `cmd/root.go:37`: `var exitIfError = errorhandler.CreateExitIfError("GSMLG CLI Error")`

### Adding New Commands
1. Create new file in `cmd/` directory
2. Define command using `cobra.Command` struct
3. Register in `init()` function with `rootCmd.AddCommand(yourCmd)`
4. Use existing commands as templates (e.g., `cmd/blog.go`)

### Semantic Release
- Uses `.releaserc.yaml` for release configuration
- Builds multi-platform binaries during release
- Outputs version info to `$GITHUB_OUTPUT` for CI integration
- Supported branches: main, next, next-major, beta (prerelease), alpha (prerelease)

## Testing
No test files currently exist in the codebase. Tests should follow Go conventions when added:
```bash
go test ./...
```
