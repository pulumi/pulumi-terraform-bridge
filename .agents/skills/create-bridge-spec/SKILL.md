---
name: create-bridge-spec
description: Use when creating, reviewing, or revising a design/spec for a non-trivial Pulumi Terraform Bridge behavior change. Applies across build-time generation, runtime SDKv2, runtime Plugin Framework, muxed providers, dynamic bridge, lifecycle/state behavior, schema metadata, docs/codegen boundaries, and downstream provider proofs. Do not use for narrow code edits where the behavior and validation path are already obvious.
---

# Create Bridge Spec

Use this skill to turn ambiguous bridge work into a compact, repo-grounded spec
or implementation plan. The bridge crosses many boundaries; the job is to make
the relevant boundaries explicit without creating a checklist of every possible
bridge subsystem.

## Core Move

Before drafting the spec, establish four things:

1. **Behavior**: what user-visible or provider-author-visible behavior is being
   changed.
2. **Path**: which real code path proves that behavior today.
3. **Ownership**: which layer owns each capability involved.
4. **Proof**: the smallest executable check that would make the change done.

If any of those are unclear, investigate the repo before writing conclusions.

## Boundary Map

Classify only the boundaries that matter for the task:

- **Build-time generation**: `pkg/tfgen`, `pkg/convert`, `pkg/tf2pulumi`,
  provider schema generation, docs conversion, metadata generation.
- **Runtime SDKv2**: `pkg/tfbridge`, `pkg/tfshim/sdk-v{1,2}`,
  `pkg/providerserver`.
- **Runtime Plugin Framework**: `pkg/pf/tfbridge`, `pkg/pf/internal/*`,
  `pkg/pf/proto`.
- **Muxing**: `pkg/x/muxer`, `pkg/pf/internal/muxer`, dispatch metadata,
  ownership split between subproviders.
- **Dynamic bridge**: `dynamic/*`, `pkg/pf/proto`, generated dynamic schema
  compatibility.
- **Cross-cutting runtime state**: lifecycle RPCs, imports, refresh, state
  upgrade, raw state, config, defaults, secrets, diagnostics.

Do not assume that one layer owns all related behavior. Schema shape, runtime
RPC handling, metadata, validation, state translation, and downstream provider
exposure can have different owners.

## Assumption Checkpoint

When the plan crosses any boundary above, pause and write down:

- The assumptions being made about routing, schema source, metadata source,
  state shape, lifecycle order, and provider mode.
- One plausible alternative assumption for each risky point.
- The quickest repo or downstream probe that can distinguish them.

Keep this short. The point is to prevent hidden assumptions from becoming a
phase plan.

## Choose The Artifact

Use the smallest artifact that reduces real risk:

- **Chat-only plan**: the behavior is narrow, the path is clear, and the
  validation proof is obvious.
- **Checked-in spec**: behavior crosses build-time/runtime boundaries, SDKv2/PF
  parity, mux ownership, dynamic bridge compatibility, lifecycle/state
  semantics, or multiple plausible designs.
- **Handoff/status doc**: the work spans sessions and has temporary branch
  state, staged rollout details, or unresolved blockers that should not clutter
  the durable spec.

Use downstream providers as proof points, not as bridge-owned inventories. A
specific provider example may prove the path, but the bridge spec should stay
provider-agnostic unless the task is explicitly provider-specific.

## Spec Shape

For a checked-in spec, prefer this compact shape:

```markdown
# <Topic> Bridge Semantics

## Summary
What is changing, who observes it, and why it matters.

## Current Flow
Code-grounded walkthrough of the relevant path. Distinguish build-time,
runtime startup, per-RPC behavior, mux dispatch, dynamic bridge behavior, and
downstream provider exposure only where relevant.

## Desired Semantics
Numbered invariants that can be tested or reviewed.

## Design
Implementation approach, capability owners, metadata/schema sources, and
shortcuts rejected.

## Compatibility Risks
Only the risks that apply: SDKv2/PF parity, muxing, aliases, dynamic providers,
state upgrade, imports, refresh, secrets, defaults, raw state, generated schema,
diagnostics timing, or downstream provider behavior.

## Validation
The smallest executable proof for each important invariant, including any
downstream provider probe needed to prove the real path.
```

Omit sections that add no information. Split durable semantics from temporary
rollout notes when combining them would make the spec noisy.

## Validation Guidance

Map each important invariant to a concrete check:

- Build-time schema/metadata: `pkg/tfgen`, `pkg/pf/tfgen`, generated schema
  tests, or golden files.
- PF runtime: `pkg/pf/tests`, `pkg/pf/internal/*`, `pkg/pf/tfbridge`.
- SDKv2 runtime and parity: `pkg/tfbridge`, `pkg/tfshim/sdk-v2`,
  `pkg/internal/tests/cross-tests`.
- Mux behavior: mux dispatch tests plus a proof that the normal downstream
  provider path exercises the intended route.
- Dynamic bridge: dynamic tests and golden files when schema compatibility can
  drift.

Prefer focused test commands first. When repo guidance says to use `mise`, use
`mise exec -- go test ...` or `mise exec -- make ...`.

## Guardrails

- Trace code facts from the repo instead of asking the user to supply them.
- Keep examples representative, not exhaustive.
- Do not hand-edit generated SDKs, generated schema artifacts, vendored
  upstream provider code, or submodule content.
- Stop and return to the user when the semantic model is unsettled, the proof
  path depends on credentials or external state, or the requested scope would
  require a broader compatibility decision than the user asked for.
