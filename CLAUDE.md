# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

This is a Go-based project using a Makefile for common tasks:

### Building
- `make build` - Build all Go packages
- `go mod tidy` - Update dependencies

### Testing
- `make test` - Run full test suite (includes installing Pulumi plugins, takes ~2 hours)
- `make test_accept` - Run tests accepting current output as golden files
- `PULUMI_ACCEPT=1 go test ./...` - Alternative way to run acceptance tests
- Run specific test: `go test -run TestSpecificTest ./path/to/package`

### Linting and Formatting
- `make lint` - Run linting across all Go modules
- `make lint_fix` - Run linting with auto-fix
- `make fmt` - Format Go code with gofmt

### Plugin Management
- `make install_plugins` - Install required Pulumi plugins for testing

### Other Commands
- `make tidy` - Run `go mod tidy` on all Go modules
- `make generate_builtins_test` - Generate builtin tests using Python scripts
- `go run scripts/build.go lint` - Alternative way to run linting
- `go run scripts/build.go fix-lint` - Alternative way to run linting with fixes

## Architecture

This is the Pulumi Terraform Bridge, which adapts Terraform Providers for use with Pulumi. The codebase consists of:

### Core Packages
- `pkg/tfgen/` - Code generation from Terraform provider schemas to Pulumi packages
- `pkg/tfbridge/` - Runtime bridge connecting Pulumi engine to Terraform providers via RPC
- `dynamic/` - Dynamic provider functionality
- `unstable/` - Experimental/unstable features

### Key Architecture Components

**Design-time vs Runtime**: The bridge operates in two phases:
1. **Design-time**: `tfgen` generates Pulumi packages by inspecting Terraform provider schemas
2. **Runtime**: The bridge connects Pulumi engine to Terraform providers using Pulumi's RPC interfaces

**Module Structure**: The project uses Go modules with multiple roots:
- Root module (`.`) - Main bridge code
- `tools/` - Development tools and utilities

**Terraform Integration**: Supports both:
- Terraform Plugin SDK v2 (legacy)
- Terraform Plugin Framework (newer)
- Mixed providers using terraform-plugin-mux

### Testing Structure
- Tests require various Pulumi plugins (AWS, GCP, Random, etc.)
- Integration tests use test providers in `internal/testing/`
- Golden file testing with `PULUMI_ACCEPT=1` for updating expected outputs

### Environment Variables
Several environment variables control `tfgen` behavior:
- `PULUMI_SKIP_MISSING_MAPPING_ERROR` - Skip errors for unmapped resources
- `PULUMI_SKIP_EXTRA_MAPPING_ERROR` - Skip errors for extra mappings
- `PULUMI_MISSING_DOCS_ERROR` - Fail on missing documentation
- `PULUMI_CONVERT` - Enable HCL to PCL conversion
- `COVERAGE_OUTPUT_DIR` - Generate conversion coverage reports

The codebase follows Go conventions with gRPC for provider communication and supports creating new bridged providers from existing Terraform providers.