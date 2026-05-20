---
name: create-bridge-spec
description: Use when planning or designing substantial Pulumi Terraform Bridge behavior changes, especially changes involving tfgen versus runtime boundaries, SDKv2 versus Plugin Framework paths, muxed providers, dynamic bridge compatibility, provider schema construction, lifecycle RPCs, state upgrade, imports, refresh, raw state, or provider upgrade regressions. Do not use for routine generated SDK churn, small local bug fixes, or straightforward docs/codegen edits unless bridge runtime semantics are being debated.
---

# Pulumi Terraform Bridge Runtime Design

Use this skill to turn ambiguous bridge behavior into a repo-grounded design or implementation plan. The goal is to preserve distinctions that are easy to blur in this repo: build-time versus runtime, SDKv2 versus Plugin Framework, static providers versus dynamic bridge, Terraform protocol behavior versus Pulumi provider behavior, and schema metadata versus live provider schemas.

## Start With The Boundary

Before drafting a plan or spec, classify the change:

- **Build-time**: `pkg/tfgen`, `pkg/convert`, `pkg/tf2pulumi`, provider schema generation, docs conversion, metadata generation.
- **Runtime SDKv2**: `pkg/tfbridge`, `pkg/tfshim/sdk-v{1,2}`, `pkg/providerserver`.
- **Runtime Plugin Framework**: `pkg/pf/tfbridge`, `pkg/pf/internal/*`, `pkg/pf/proto`.
- **Muxing**: `pkg/x/muxer`, `pkg/pf/internal/muxer`, dispatch metadata.
- **Dynamic bridge**: `dynamic/*`, `pkg/pf/proto`, generated dynamic schema compatibility.
- **Cross-cutting**: state upgrade, raw state delta, naming, secrets, defaults, config encoding, diagnostics.

Do not ask the user to explain code facts that can be read from the repo. Trace the current flow first, then decide what should change.

## Required Context

For substantial runtime or schema behavior, inspect and summarize:

- The issue, PR, failing test, downstream provider regression, or memory/performance measurement motivating the work.
- Which Pulumi RPCs are involved: `CheckConfig`, `Configure`, `Check`, `Diff`, `Create`, `Read`, `Update`, `Delete`, `Invoke`, import, refresh, or state upgrade.
- Whether the behavior must match Terraform, prior Pulumi bridge behavior, generated Pulumi schema, SDKv2 behavior, PF behavior, or dynamic bridge compatibility.
- The schema source being used: Terraform SDK schema, PF provider/resource/data source schema, generated Pulumi package schema, bridge metadata, mux dispatch table, or dynamic provider schema.
- The local code path with file/line references.

## Decide The Artifact

Use the smallest artifact that reduces real risk:

- **Chat-only plan**: narrow behavior change with obvious implementation and low compatibility risk.
- **Checked-in design spec**: cross-cutting behavior, lifecycle semantics, tfgen/runtime boundary changes, SDKv2/PF parity questions, dynamic bridge compatibility, or multiple plausible models.
- **Handoff/status doc**: multi-session work where decisions, blockers, and verification state must survive context loss.

Prefer one bridge behavior/design spec over separate product and tech specs unless the user explicitly wants that split.

## Spec Shape

When a checked-in spec is warranted, use a compact Markdown document with these sections:

```markdown
# <issue-or-topic> Bridge Runtime Semantics

## Summary
What behavior is wrong, what semantic model should replace it, and which issue/PR this serves.

## Current Flow
Code-grounded walkthrough. Distinguish tfgen/build-time, runtime startup, per-RPC runtime behavior, mux dispatch, and dynamic bridge behavior.

## Desired Semantics
Numbered, testable invariants from the perspective of provider authors, Pulumi users, the Pulumi engine, generated SDKs, Terraform protocol servers, and future bridge maintainers.

## Validation Boundary
What runs at build time, what runs at runtime startup, and what runs per user operation.

## Design
Implementation approach that fits the existing bridge. Name key modules, ownership boundaries, metadata used, and rejected shortcuts.

## Compatibility Risks
SDKv2/PF parity, muxing, aliases, dynamic providers, state upgrade, imports, refresh, secrets, defaults, raw state, generated schema stability, and diagnostics timing.

## Test Plan
Targeted unit tests, PF/SDKv2 runtime tests, cross-tests, generated schema checks, dynamic bridge golden checks, downstream provider proof, and memory/performance benchmarks.
```

Omit sections only when they truly add no information.

## Domain Rules

- Treat **tfgen schema generation** and **runtime provider execution** as separate phases. Do not use runtime validation or provider schema availability as proof of build-time coverage without tracing both paths.
- For PF changes, identify whether the behavior depends on `providerserver.NewProtocol6`, `GetProviderSchema`, `GetMetadata`, per-resource schema loading, or bridge schema adapters.
- For SDKv2 changes, check the SDK shim and Terraform SDK behavior before assuming a PF rule applies.
- For muxed providers, prove whether the SDKv2-only path pays PF costs, and whether dispatch/alias resolution requires schema or only metadata.
- For dynamic bridge changes, check generated schema compatibility and dynamic golden tests before accepting schema drift.
- For lifecycle changes, trace the specific RPC path and the state shape it consumes or returns.
- For refresh/import questions, keep refresh from existing state separate from import-style reads.
- For state upgrade and raw state, inspect `__meta`, `RawStateDelta`, private state, and `UpgradeResourceState`.
- Do not hand-edit generated SDKs, generated schema artifacts, vendored upstream provider code, or submodule content.

## Validation

Map every important invariant to concrete verification:

- Use targeted tests in `pkg/pf/tests`, `pkg/pf/internal/*`, or `pkg/pf/tfbridge` for PF runtime behavior.
- Use targeted tests in `pkg/tfbridge`, `pkg/tfshim/sdk-v2`, or `pkg/internal/tests/cross-tests` for SDKv2 and parity behavior.
- Use `pkg/tfgen` and `pkg/pf/tfgen` tests for build-time schema and metadata behavior.
- Use dynamic bridge tests and golden files when schema output compatibility could change.
- Prefer focused commands such as `go test ./pkg/pf/tests -run '<TestName>' -count=1` before broad suites.
- When the repo guidance says to use `mise`, use `mise exec -- make ...` or `mise exec -- go test ...` instead of bare commands.

## Stop Conditions

Stop and return to the user when the semantic model is not settled, when SDKv2 and PF compatibility require different user-visible behavior, when dynamic bridge compatibility is unclear, or when the implementation would require generated schema or SDK behavior changes beyond the requested issue.
