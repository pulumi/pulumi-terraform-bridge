---
name: create-bridge-spec
description: Use when creating, reviewing, or revising a design/spec for a non-trivial Pulumi Terraform Bridge behavior change. Applies across build-time generation, runtime SDKv2, runtime Plugin Framework, muxed providers, dynamic bridge, lifecycle/state behavior, schema metadata, docs/codegen boundaries, and downstream provider proofs. Do not use for narrow code edits where the behavior and validation path are already obvious.
---

# Create Bridge Spec

Use this skill to find the bridge boundary that owns the reported behavior and
preserve only the decisions a durable spec needs. It is a routing aid, not a
required planning framework or a checklist of bridge subsystems.

Read [`AGENTS.md`](../../../AGENTS.md) and the
[architecture overview](../../../docs/architecture/overview.md), following only
the guides relevant to the reported behavior. Trace code facts from the
repository instead of asking the user to supply them.

## Ground The Change

Before drafting a spec—or deciding that one is needed—establish:

1. **Behavior**: the concrete provider-author or Pulumi-user behavior changing.
2. **Path**: the mode, lifecycle transition, and current code path that exhibit
   it.
3. **Ownership**: the layer that owns each capability involved.
4. **Proof**: the smallest executable check that would make the change done.

Investigate any uncertainty that would materially change ownership, support
scope, or design. Name the assumption, plausible alternative, and quickest repo
or downstream probe that distinguishes them. Do not turn clear boundaries into
an assumption-analysis ritual.

## Keep The Change Narrow

- Propose the smallest end-to-end change that fixes the motivating path.
- Treat adjacent build-time, SDKv2, PF, mux, dynamic, and lifecycle cases as
  compatibility questions to disposition, not automatic requirements to
  implement.
- Preserve existing behavior in adjacent or unsupported paths when stronger
  shared semantics cannot be established safely.
- Before adding generic machinery for an uncommon case, check whether it occurs
  in current bridge paths, tests, downstream providers, or user reports. Record
  its incidence, safe fallback, and possible follow-up when support is costly.
- Broaden across SDKv2 and PF, muxing, dynamic bridge, or downstream providers
  only when the motivating behavior depends on their agreement or an existing
  compatibility contract requires it. Otherwise document why the behavior is
  mode-specific.
- Keep downstream providers as focused proof points, not bridge-owned
  inventories.

## Choose The Artifact

Use the conversation when behavior, path, ownership, and proof are clear. Do not
create a design document merely because this skill was loaded.

Create a checked-in spec when the user asks for one or unresolved semantic,
ownership, lifecycle, or compatibility decisions must survive review or
handoff. Crossing a repository boundary by itself is not enough. Use a temporary
handoff/status note instead when only branch state, rollout progress, or blockers
need to survive another session.

## Keep The Spec Compact

Include only sections that preserve a decision or make the implementation
reviewable:

- **Summary**: what changes, who observes it, and why it matters.
- **Current Path**: the relevant build-time or runtime flow grounded in code.
- **Desired Semantics**: testable behavior and explicit non-goals.
- **Design**: capability owners, data or schema sources, and rejected shortcuts.
- **Compatibility Dispositions**: only applicable modes and lifecycle risks,
  each marked supported now, unaffected, existing fallback, explicitly
  unsupported/non-goal, or deferred.
- **Validation**: the smallest proof for each important invariant.

Omit sections that add no information. Keep temporary rollout notes out of the
durable semantics.

## Proof And Guardrails

- Prefer one focused test that proves the user-visible or
  provider-author-visible regression.
- Add lower-level, cross-mode, golden, or downstream tests only when they provide
  distinct proof or useful failure localization. The test locations in
  `AGENTS.md` are routing options, not a required matrix.
- Use repository-prescribed `mise` and test commands. Start focused before
  broadening validation.
- Do not hand-edit generated SDKs, generated schema artifacts, vendored upstream
  provider code, or submodule content.

Return to the maintainer with concrete options when the semantic owner or safe
fallback is unclear, the implementation is becoming substantially broader than
the reported behavior, a rare case requires disproportionate complexity, a
broader compatibility decision is needed, or required proof depends on
credentials or external state unavailable in the current environment. Report
the evidence gap and concrete validation options.
