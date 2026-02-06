# AGENTS

This repo is the Pulumi Terraform Bridge. Keep changes small, focused, and consistent with the architecture below.

## What this repo does
- Build-time generation: translate Terraform provider schemas into Pulumi schema/SDKs.
- Runtime translation: act as a Pulumi provider while driving a Terraform provider under the hood.
- These two pipelines are tightly coupled; changes in one can affect the other.

## Major components (quick map)
- Build-time: `pkg/tfgen`, `pkg/convert`, `pkg/tf2pulumi`
- Runtime (SDKv2): `pkg/tfbridge`, `pkg/tfshim/sdk-v{1,2}`, `pkg/providerserver`
- Runtime (PF): `pkg/pf`, `pkg/tfshim/schema`
- Muxing: `pkg/x/muxer`
- Dynamic bridge: `dynamic/*`
- Tests: `pkg/tests`, `pkg/internal/tests/cross-tests`, `pkg/pf/tests`
- Ops: `Makefile`, `scripts/`

## Workflow guidance
- Identify whether a change is build-time, runtime, or both. Update tests that cover the affected path.
- Favor local changes near the relevant layer; avoid cross-layer coupling unless required.
- Preserve existing behavior of diff/plan/state unless explicitly changing semantics.
- If adding new runtime behavior, confirm it works for both SDKv2 and PF paths (or document why it is specific).

## Useful entry points
- Build-time schema: `pkg/tfgen/schema.go`, `pkg/tfgen/generate.go`, `pkg/tfgen/docs.go`
- Runtime provider (SDKv2): `pkg/tfbridge/provider_*.go`, `pkg/tfbridge/diff.go`, `pkg/tfbridge/detailed_diff.go`
- Runtime provider (PF): `pkg/pf/tfbridge/provider_*.go`, `pkg/pf/tfbridge/provider_diff.go`
- Cross-cutting: `pkg/tfbridge/state.go`, `pkg/tfbridge/secrets.go`, `pkg/convert/secret.go`, `internal/logging`

## Tests
- Prefer targeted tests in the existing harness for the mode you touch.
- Common locations: `pkg/tests`, `pkg/internal/tests/cross-tests`, `pkg/pf/tests`.

## Docs
- Architecture overview: `docs/architecture/overview.md`
- Guides: `docs/guides/*`

## Commands
- Build: `make build`
- Lint: `make lint`
- Test: `make test`
