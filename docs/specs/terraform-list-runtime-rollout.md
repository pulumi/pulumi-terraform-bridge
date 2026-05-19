# Terraform List Runtime Rollout

## Purpose

This document tracks the branch-local and stacked-PR work needed to reach the
end-state semantics in `docs/specs/terraform-list-runtime-semantics.md`.

The semantics spec is intentionally durable and includes muxed-provider support.
This rollout document is allowed to be temporary: it names the current branch
state, the known implementation gaps, and the order in which the work should
land.

## Current Branch State

Branch `chall/list-runtime-spec` is staging the first implementation slice from
the `fraser/list` work, repaired against the semantics in this spec set.

The `fraser/list` code has the right broad shape for a single PF provider:

- schema-only list-resource discovery
- `listInputs` generation
- PF provider `ListWithContext`
- provider-owned continuation sessions
- panic recovery for the streaming `List` RPC

Those commits do not yet satisfy the end-state semantics. The next work should
be framed as implementation repair and staging, not greenfield design.

## Phase 1: PF Runtime And PF-Owned Mux Support

Goal: land coherent list support for PF-owned resources, including the normal
muxed-provider path used by providers such as `pulumi-aws`, with unsupported
cases failing closed.

Phase 1 is done only when a live `pulumi-aws` test can call Pulumi `List`
through the provider's normal muxed server for a PF-owned resource. A direct
non-muxed PF provider test is useful but is not sufficient.

Recommended v1 ID scope:

- Request resource-object list results with `IncludeResource: true`.
- Use a concrete top-level resource object `id` when Terraform returns one.
- If no resource object `id` is available, decode Terraform identity data with
  the resource identity schema.
- If the identity data has an attribute named `id`, stringify that value.
- For one identity attribute, stringify that value as `ListResponse.Result.Id`.
- For multiple identity attributes, sort attributes by name, stringify each
  value, and join the values with `,`.
- Treat identity-derived IDs as a deterministic Pulumi-owned list result format,
  not as proof that Terraform import accepts the same string.
- Fail only when no resource object `id` is available and identity data is
  missing, cannot be decoded, or cannot be stringified deterministically.

AWS evidence for this v1 scope:

- The initial scope reaches PF-owned resources through the normal muxed
  provider path. It does not yet reach SDKv2-owned resources whose list
  implementation is exposed by the framework sidecar.
- AWS currently has `32` `@FrameworkListResource` entries.
- `10` have an identity attribute named `id`.
- `16` more have a single non-`id` identity parameter or ARN.
- This v1 rule uses a single identity value for `26 / 32` AWS Framework list
  resources.
- The remaining `6` AWS Framework list resources have compound identities and
  use the sorted comma-joined Pulumi ID format:
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
- Support the identity-derived ID path described above, including the sorted
  comma-joined format for compound identities.
- Return terminal errors after partial results instead of sending a successful
  empty continuation.
- Bound provider-side buffering for unbounded Pulumi `limit`.
- Add mux `List` forwarding for resources already owned by the PF subprovider
  according to the existing resource dispatch table.
- Keep SDKv2-owned resources whose list support lives on the PF side
  unimplemented until list-specific mux ownership metadata exists.

Minimum tests:

- PF provider without list resources constructs successfully.
- `listInputs` is emitted for list resources and omitted for non-list resources.
- Empty list config schema emits empty non-nil `listInputs`.
- Unknown query returns `Computed` and does not call Terraform.
- Concrete query calls `ValidateListResourceConfig` before `ListResource`.
- Validation diagnostics fail before session creation.
- Continuation token rejects changed token, query, or limit.
- Resource-object result with a top-level `id` uses that value without
  `extractID` or `ComputeID`.
- Identity-only result with an `id` attribute uses that value.
- Identity-only result with one attribute produces a list result ID from that
  attribute value.
- Identity-only result with compound identity uses the sorted comma-joined
  Pulumi ID format.
- Terminal error after partial results returns an error.
- `limit` and `page_size` boundary cases cover overproduction and unbounded
  limit behavior.
- Muxed `List` routes a PF-owned resource token through the existing resource
  dispatch table.
- Live downstream proof: `pulumi-aws` can call `List` through
  `testProviderServer()` for a PF-owned resource token.
- SDKv2-owned list resources in muxed providers still fail closed rather than
  accidentally routing by Terraform list ownership that Phase 1 does not yet
  model.

Suggested verification:

```bash
mise exec -- go test -count=1 ./pkg/pf/internal/schemashim ./pkg/pf/tfgen ./pkg/pf/tfbridge -run 'Test.*List|Test.*ListResource'
mise exec -- go test -count=1 ./pkg/pf/tests -run 'TestPFCheckConfig|TestCallWithTerraformConfig'
mise exec -- go test -count=1 ./pkg/providerserver -run TestPanicRecoveringProviderServer_List
mise exec -- go test -count=1 ./pkg/x/muxer/... -run 'TestList|TestSimpleDispatch'
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

Goal: route Pulumi `List` to the subprovider that owns Terraform list support
when that owner differs from the managed-resource CRUD owner.

Required work:

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

Identity-aware import/read remains out of scope. Phase 1 returns deterministic
Pulumi-owned list IDs for decoded Terraform identities, but those strings are
not guaranteed to match a provider's Terraform string import-ID format.

See `docs/specs/terraform-list-identity-review.md` for the current identity
findings and AWS provider evidence.

## Current Known Mismatches

These are known gaps between the current implementation commits and the
end-state spec:

- Non-list PF providers can panic during schema-only shim construction.
- The muxer has no list-owner dispatch table for resources whose CRUD owner and
  Terraform list owner differ.
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
