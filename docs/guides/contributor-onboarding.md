# Contributor Onboarding

Welcome! This guide distills the minimum context required to make your first change to the Pulumi Terraform Bridge. It
assumes you are comfortable with Go and have Pulumi/Terraform installed, but are new to this repository.

## Prerequisites

- Go version from `go.mod` (install via `mise` or your package manager).
- Pulumi CLI ≥ 3.x (required for tests and plugin installs).
- Terraform / OpenTofu CLI (helps when debugging or running cross-tests locally).

Optional but recommended:

- `mise` (see `mise.toml`) to manage tool versions.
- Docker (some tests rely on Dockerized providers; note restrictions in `pkg/tests` if you run into issues).

## Getting Set Up

1. Clone the repo and optionally run `make go.work` to set up a Go workspace that includes nested modules.
2. Run `make build` once; it tidies modules and ensures packages compile.
3. Run `make test RUN_TEST_CMD=./pkg/tfbridge -run TestProviderPreview` (or another targeted test) to validate your local
   environment before tackling larger changes. Full `make test` is heavy (downloads plugins and runs integration suites).

## Repository Tour

- **Runtime bridge** – `pkg/tfbridge`, `pkg/tfshim`: Pulumi RPC handlers + Terraform shims.
- **Plugin Framework support** – `pkg/pf` and related shims.
- **Hybrid / muxing runtime** – `pkg/x/muxer` composes SDKv2 + PF providers during migrations.
- **Build-time tooling** – `pkg/tfgen`, `pkg/convert`, `pkg/tf2pulumi`.
- **Dynamically bridged provider** – `dynamic/` binary that parameterizes arbitrary Terraform providers.
- **Tests** – `pkg/tests` (schema + program), `pkg/internal/tests/cross-tests`, `pkg/pf/tests`, plus replay/unit suites.
- **Documentation** – `docs/` (Sphinx + Markdown), with guides referenced from the main README.

## Common Workflows

| Task | Steps |
| ---- | ----- |
| Add/adjust runtime behavior | Update `pkg/tfbridge/*` or `pkg/pf/tfbridge/*`, add focused tests under matching package, run targeted Go tests. |
| Update shims | Touch `pkg/tfshim/sdk-v2/*` (or PF equivalents), run `make test RUN_TEST_CMD=./pkg/tfshim/sdk-v2`. |
| Modify TFGen output | Edit `pkg/tfgen/*`, verify with unit tests and golden fixtures (`make test RUN_TEST_CMD=./pkg/tfgen`). |
| Improve docs | Update Markdown in `docs/` and re-run Sphinx locally (`cd docs && make html`) if changing the rendered site. |
| Accept golden updates | After intentionally changing output, run `make test_accept` and review diffs carefully. |

## Debugging Tips

- **Pulumi <-> Provider traffic** – Use `PULUMI_DEBUG_GRPC=1` during tests to inspect gRPC payloads.
- **Terraform behavior** – Reproduce using Terraform CLI or cross-tests to confirm parity gaps.

## Coding Standards

- Run `make fmt` (gofmt -s) and `make lint` (golangci-lint) before sending a PR.
- Favor explicit module paths under `github.com/pulumi/pulumi-terraform-bridge/v3/...`.
- Wrap returned errors with context: `fmt.Errorf("failed to diff %s: %w", name, err)`.
- Avoid `panic` in library code; prefer returning errors.
- Tests should be table-driven when practical and run in parallel (`t.Parallel()`).

## Sending Your First PR

1. Create or update tests that demonstrate the behavior change.
2. Run targeted tests locally; include the command(s) in your PR description.
3. Fill out the PR template with motivation, testing, and any follow-up tasks.
4. Tag maintainers familiar with the relevant area
