# Pulumi Terraform Bridge Architecture Overview

This document captures the high–level mental model maintainers use to reason about the Pulumi Terraform Bridge. It is
intended as a launchpad: each section links to the code, docs, or playbooks that provide deeper detail.

## System Goals

The bridge has two equally important responsibilities:

1. **Build-time generation** – Translate a Terraform provider's schema and metadata into a Pulumi schema, SDKs, and docs so Pulumi users can program against the provider.
2. **Runtime translation** – Act as a Pulumi resource provider that speaks Pulumi's gRPC protocol while driving the
   Terraform provider implementation under the hood.

A change to either half can impact the other. Treat the bridge as one product with two tightly-coupled pipelines.

## Major Components

| Layer | Location | Role |
| ----- | -------- | ---- |
| Build-time | `pkg/tfgen`, `docs/guides/*`, `pkg/convert`, `pkg/tf2pulumi` | Introspect Terraform provider schemas, emit Pulumi schema, SDKs, and docs. |
| Runtime (SDKv2) | `pkg/tfbridge`, `pkg/tfshim/sdk-v{1,2}`, `pkg/providerserver` | Drive Terraform Plugin SDK providers via Pulumi RPC entry points. |
| Runtime (PF) | `pkg/pf/*`, `pkg/tfshim/schema`, `pkg/x/muxer` | Bridge Terraform Plugin Framework providers and mixed muxed providers. |
| Dynamic bridge | `dynamic/*` | Parameterizable provider that downloads and hosts Terraform providers at runtime. |
| Testing | `pkg/tests`, `pkg/internal/tests/cross-tests`, `pkg/pf/tests` | Harnesses that keep behavior aligned with Terraform and Pulumi expectations. |
| Ops & Tooling | `Makefile`, `scripts/`, `docs/operations` | Build, lint, test, release, and operational playbooks. |

## Build-Time Pipeline

```
Terraform Provider ──► Schema Introspection (`tfgen/schema.go`)
      │                 │
      │                 ├─► Pulumi PackageSpec (`pkg/tfgen/generate.go`)
      │                 ├─► Language SDKs (`pkg/tfgen/generate_<lang>.go`)
      │                 └─► Docs + Examples (`pkg/tfgen/docs.go`, `pkg/convert`)
      ▼
Pulumi SDK + Schema Artifacts
```

Key extension points:

- **ProviderInfo overlays** (`pkg/tfbridge/info`) describe renames, doc overrides, examples, and custom CRUD behavior.
- **Edit rules** (`pkg/tfgen/edit_rules.go`) post-process generated schema and docs.
- **Converters** (`pkg/convert/*`) translate Terraform values to Pulumi `PropertyValue`s.

### Common Maintenance Tasks

- Adding a new Terraform resource → update ProviderInfo + rerun `tfgen` via provider repo.
- Debugging doc generation → reproduce with `make test RUN_TEST_CMD=./pkg/tfgen` and inspect `COVERAGE_OUTPUT_DIR` output.

## Runtime Pipeline

```
Pulumi Engine ─► `pkg/tfbridge.Provider` ─► `pkg/tfshim` ─► Terraform Provider
            (gRPC)           (resource RPC)        (SDKv{1,2} or PF protocol)
```

Responsibilities:

- **Lifecycle RPCs** – `Create`, `Read`, `Update`, `Delete`, `Invoke`, `Plan`, `CheckConfig`, etc. live in
  `pkg/tfbridge/provider_*.go` with shared helpers in `pkg/tfbridge/diff.go`, `pkg/tfbridge/schema.go`, etc.
- **Shim layers** – `pkg/tfshim/sdk-v2` (and legacy `sdk-v1`) normalize Terraform Plugin SDK APIs; PF runtime uses
  `pkg/pf/tfbridge` alongside `pkg/tfshim/schema`.
- **Muxing / Migration** – `pkg/x/muxer` combines SDKv2 + PF implementations during migrations.
- **Panic recovery** – `pkg/providerserver/panic_recovering_provider.go` guards against provider panics.
- **Diff & plan semantics** (`pkg/tfbridge/diff.go`, `pkg/pf/tfbridge/provider_diff.go`).
### Cross-Cutting Concerns

- **State translation** (`pkg/tfbridge/state.go`, `pkg/tfshim/sdk-v2/upgrade_state.go`).
- **Secrets & defaults** (`pkg/convert/secret.go`, `pkg/tfbridge/secrets.go`).
- **Logging** (`internal/logging`).

## Bridging Modes

| Mode | When to use | Entry points |
| ---- | ----------- | ------------ |
| **Static SDKv2 Bridge** | Terraform providers built on Plugin SDK v2. | `pkg/tfbridge`, `pkg/tfshim/sdk-v2`. |
| **Static PF Bridge** | Providers authored with Plugin Framework. | `pkg/pf/tfbridge`, `pkg/pf/provider.go`. |
| **Muxed Bridge** | Hybrid providers migrating resource-by-resource. | `pkg/x/muxer`, guides under `docs/guides/upgrade-sdk-to-mux.md`. |
| **Dynamic Bridge** | Parameterize any registry provider at runtime. | `dynamic/main.go`, `dynamic/internal/shim`. |

Each mode shares testing harnesses but has mode-specific fixtures (see `docs/guides/testing.md`).

## Operational Flows

- **Build & test** – `make build`, `make lint`, `make test`. See `AGENTS.md` and `docs/guides/contributor-onboarding.md`.
- **Releases** – Follow platform providers playbook (link in root README); capture bridge-specific steps in
  `docs/operations/maintainability-playbook.md`.

## Where to Learn More

- Runtime deep dive (planned) → `docs/architecture/runtime.md` (placeholder).
- Build-time cookbook → `docs/guides/new-provider.md`, `docs/guides/new-pf-provider.md`.
- Testing strategy → `docs/guides/testing.md` (this repo) + `pkg/tests` examples.
- Maintaining TODOs & deprecations → `docs/operations/todo-triage.md`.

_Questions, gaps, or corrections? Add inline `<!-- TODO(owner): ... -->` comments or open an issue under the
`docs` label so we track updates._
