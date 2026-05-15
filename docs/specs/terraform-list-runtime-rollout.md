# Terraform List Runtime Rollout

## Purpose

This document tracks the branch-local and stacked-PR work needed to reach the
end-state semantics in `docs/specs/terraform-list-runtime-semantics.md`.

The semantics spec is intentionally durable and includes muxed-provider support.
This rollout document is allowed to be temporary: it names the current branch
state, the known implementation gaps, and the order in which the work should
land.

## Current Branch State

Branch `chall/list-runtime-spec` is a specs-only planning branch. The
implementation work should start by cherry-picking the existing commits from
`fraser/list`, then repairing and staging that implementation against the
semantics in this spec set.

The `fraser/list` code has the right broad shape for a single PF provider:

- schema-only list-resource discovery
- `listInputs` generation
- PF provider `ListWithContext`
- provider-owned continuation sessions
- panic recovery for the streaming `List` RPC

Those commits do not yet satisfy the end-state semantics. The next work should
be framed as implementation repair and staging, not greenfield design.

## Phase 1: Single-Provider PF Support

Goal: land coherent list support for a non-muxed PF provider, with unsupported
cases failing closed.

Recommended v1 identity scope:

- Support identity-only results only when the Terraform resource identity schema
  has exactly one required scalar identity attribute.
- Decode the Terraform identity value and use that single attribute value as
  `ListResponse.Result.Id`.
- Treat this as a narrow string-ID compatibility shortcut, not the end-state
  identity model.
- Fail closed for compound identities, missing identity schemas, non-scalar
  identity values, and identity-only results that cannot be decoded.

AWS evidence for this v1 scope:

- The initial non-muxed scope only reaches `@FrameworkListResource`
  implementations.
- AWS currently has `32` `@FrameworkListResource` entries.
- `10` have an identity attribute named `id`.
- `16` more have a single non-`id` identity parameter or ARN.
- This v1 rule covers `26 / 32` AWS Framework list resources.
- The remaining `6` AWS Framework list resources have compound identities and
  should fail closed:
  - `aws_dynamodb_global_secondary_index`
  - `aws_msk_topic`
  - `aws_wafv2_web_acl_rule`
  - `aws_workmail_domain`
  - `aws_workmail_group`
  - `aws_workmail_user`

Required fixes:

- Make providers without `ProviderWithListResources` safe. `GatherListResources`
  should return an empty collection or `newSchemaOnlyListResourceMap` should be
  nil-safe.
- Remove runtime fallback from Terraform list schemas to managed resource
  schemas. Runtime `List` should only accept resources with real list-resource
  schemas.
- Replace `query.AsMap()` conversion with Pulumi property unmarshalling that
  preserves unknowns.
- Return exactly one `ListResponse.Computed` for query values containing
  unknowns or computed values, without calling Terraform validation or list.
- Call `ValidateListResourceConfig` before creating or exposing a continuation
  session.
- Bind continuation tokens to the original Pulumi token, normalized query, and
  limit.
- Support the narrow single-required-scalar identity-only result path described
  above, and fail closed for other identity-only Terraform list results unless
  the read/import path gains identity-aware support.
- Return terminal errors after partial results instead of sending a successful
  empty continuation.
- Bound provider-side buffering for unbounded Pulumi `limit`.

Minimum tests:

- PF provider without list resources constructs successfully.
- `listInputs` is emitted for list resources and omitted for non-list resources.
- Empty list config schema emits empty non-nil `listInputs`.
- Unknown query returns `Computed` and does not call Terraform.
- Concrete query calls `ValidateListResourceConfig` before `ListResource`.
- Validation diagnostics fail before session creation.
- Continuation token rejects changed token, query, or limit.
- Identity-only result with one required scalar identity attribute produces a
  list result ID from that attribute value.
- Identity-only result with compound identity fails closed.
- Resource-object result extracts the importable ID used by `Read`.
- Terminal error after partial results returns an error.
- `limit` and `page_size` boundary cases cover overproduction and unbounded
  limit behavior.

Suggested verification:

```bash
mise exec -- go test -count=1 ./pkg/pf/internal/schemashim ./pkg/pf/tfgen ./pkg/pf/tfbridge -run 'Test.*List|Test.*ListResource'
mise exec -- go test -count=1 ./pkg/pf/tests -run 'TestPFCheckConfig|TestCallWithTerraformConfig'
mise exec -- go test -count=1 ./pkg/providerserver -run TestPanicRecoveringProviderServer_List
git diff --check origin/main..HEAD
```

## Phase 2: Mux Schema And Dispatch Metadata

Goal: represent list-resource ownership separately from managed-resource CRUD
ownership.

Required work:

- Extend PF mux schema discovery to carry list-resource maps from each
  subprovider.
- Extend dispatch metadata with a list-resource owner map keyed by Pulumi
  resource token.
- Preserve `listInputs` from the list owner when the CRUD owner differs from
  the list owner.
- Keep SDKv2-only resources without Terraform list resources unlistable.

Minimum tests:

- Muxed resource whose CRUD owner and list owner differ emits `listInputs` from
  the list owner on the final Pulumi resource spec.
- Existing resource and function dispatch remains unchanged.
- Missing list owner produces nil `listInputs`, not accidental listability.

## Phase 3: Mux Runtime List Dispatch

Goal: route Pulumi `List` to the subprovider that owns Terraform list support.

Required work:

- Add streaming `List` forwarding to `pkg/x/muxer`.
- Route by list dispatch table, not by managed-resource dispatch.
- Return `Unimplemented` when a resource token has no list owner.
- Prove an SDKv2-managed resource exposed through a framework list wrapper can
  be listed through the mux path.

Minimum tests:

- SDKv2-only resource without Terraform list support returns `Unimplemented`.
- Muxed SDK-backed list resource dispatches to the PF/framework list owner.
- CRUD dispatch and list dispatch may target different subproviders for the
  same Pulumi resource token.

## Deferred: Identity-Aware Import Or Read

General and compound identity-only Terraform list results remain out of scope
until the bridge has a read/import path that can decode Terraform identity data
and call the correct identity-aware Terraform protocol. Until then, compound
identity-only list results must fail closed with a clear error.

See `docs/specs/terraform-list-identity-review.md` for the current identity
findings, AWS provider evidence, and open review questions.

## Current Known Mismatches

These are known gaps between the current implementation commits and the
end-state spec:

- Non-list PF providers can panic during schema-only shim construction.
- The muxer has no `List` method and no list-owner dispatch table.
- Identity-only results currently synthesize `identity:` IDs.
- Continuation tokens are looked up by opaque token only.
- Query conversion uses `structpb.Struct.AsMap()` and loses Pulumi unknowns.
- `ValidateListResourceConfig` is not called before session creation.
- Terminal errors after partial results can be swallowed.
- Runtime can fall back from `ListResourceSchemas` to managed
  `ResourceSchemas`.
- Unbounded `limit` can allow unbounded provider-side buffering.

## Review Rule

Before merging any phase, compare the final diff against
`docs/specs/terraform-list-runtime-semantics.md`. If the phase intentionally
does not implement part of the end-state, the code should fail closed for that
case and this rollout document should say which follow-up phase owns it.
