# Terraform List Runtime Semantics

## Summary

This spec defines the end-state semantics for exposing Terraform protocol list
resources through Pulumi `List`. The target scope is all Terraform protocol
list resources exposed by an upstream provider server, not only resources whose
CRUD implementation is Plugin Framework-native.

Plain SDKv2 `helper/schema` providers do not expose a real list-resource
implementation, but muxed providers can expose list resources for SDKv2-managed
resources through a Plugin Framework sidecar. For example,
terraform-provider-aws registers `@SDKListResource` implementations that wrap
SDKv2 `ResourceData` and are served from its framework provider in a
protocol-v5 mux. The bridge must support those list resources too.

The implementation may land in stacked phases. Earlier phases may implement a
strict subset of this spec, but they must fail closed for unsupported cases and
must not ship behavior that contradicts the end-state contract.

## Current Flow

### Build-Time Schema Generation

PF schema discovery gathers list resources separately from managed resources:

- `pkg/pf/internal/pfutils/listresources.go` calls
  `provider.ProviderWithListResources.ListResources` when available.
- `pkg/pf/internal/schemashim/schemashim.go` stores those schemas in
  `SchemaOnlyProvider`.
- `pkg/pf/internal/schemashim/provider.go` exposes them through
  `ListResourcesMap`.

The shared tfgen path then asks the shim provider for a list-resource schema:

- `pkg/tfgen/generate.go` finds a list schema for the Terraform resource name
  and records list input variables.
- `pkg/tfgen/generate_schema.go` emits `ResourceSpec.ListInputs` when
  `listprops` is non-nil.
- `pkg/tfgen/internal/paths/paths.go` adds a `listInputs` path kind so nested
  types get stable schema paths.

This means generated `listInputs` are derived from the Terraform list resource
config schema, not from managed resource inputs. A resource with a list
resource and an empty list config schema should still emit an empty, non-nil
`listInputs` object. A provider without list resources should emit no
`listInputs` and must not panic.

Muxed providers complicate ownership. CRUD for a Pulumi resource may be served
by one subprovider while the Terraform list resource for the same Terraform
type is exposed by another subprovider. In terraform-provider-aws, many managed
resources remain SDKv2 resources, but their list resources are exposed by the
framework side of the upstream mux. The bridge schema path therefore needs
list-resource ownership, not only managed-resource ownership.

### Runtime Startup

`pkg/pf/tfbridge/provider.go` constructs the runtime provider from the PF shim
provider. The provider records:

- managed resource schemas in `p.resources`
- data source schemas in `p.datasources`
- PF protocol v6 server in `p.tfServer`
- list continuation state in `p.listSessions`

The runtime resolves the Pulumi token to the Terraform list resource owner and
uses the Terraform list resource config schema for query conversion and
validation. It must not infer a list config schema from the managed resource
schema merely because the Terraform type names match.

The bridge muxer computes dispatch for managed resources and functions. List
support needs additional list-specific dispatch state so `List` can route to
the subprovider that exposes `ListResource`, even when CRUD routes to a
different subprovider.

### Per-RPC Runtime

`pkg/pf/internal/plugin/provider_server.go` forwards Pulumi `List` to
`ProviderWithContext.ListWithContext`.

For a first page, the bridge should:

1. Validate non-negative `limit` and `page_size`.
2. Map the Pulumi token to the Terraform list resource owner and name.
3. Find the Terraform list config schema.
4. Convert `ListRequest.query` with Pulumi unknown preservation.
5. Return `Computed` without calling Terraform when query contains unknowns.
6. Validate concrete config with Terraform `ValidateListResourceConfig`.
7. Start a provider-owned session and continuation token.
8. Call Terraform `ListResource` with `IncludeResource: true`.
9. Convert each Terraform list result into a Pulumi `ListResponse.Result`.
10. Stream up to `page_size` results and then a continuation marker.

For continuation pages, the bridge must validate that the continuation token is
being used with the same Pulumi token, normalized query, and `limit` as the
first request.

### Panic Recovery

`pkg/providerserver/panic_recovering_provider.go` wraps the streaming `List`
RPC so provider panics are logged with method context before being re-thrown.

## Desired Semantics

1. Providers that do not expose Terraform list resources are valid. They expose
   no listable resources and must not fail schema generation, provider
   construction, or unrelated tests.

2. `listInputs` is generated only for Pulumi resources whose Terraform resource
   has a matching Terraform list resource. Empty list config schemas produce an
   empty non-nil `listInputs` object. Missing list resources produce nil
   `listInputs`.

3. The Terraform list config schema is the source of truth for the Pulumi query
   shape. Managed resource schemas are only used to decode resource-object list
   results and extract Pulumi IDs.

4. A first `List` call with unknown or computed values anywhere in `query`
   returns exactly one `ListResponse.Computed` and does not call Terraform
   `ValidateListResourceConfig` or `ListResource`.

5. Concrete `query` values are converted through Pulumi property unmarshalling
   with unknown preservation, then encoded to the Terraform list config type.
   Direct `structpb.Struct.AsMap` conversion is not sufficient because it loses
   Pulumi computed markers.

6. Concrete first-page requests call Terraform `ValidateListResourceConfig`
   before starting a list session. Validation diagnostics with errors fail the
   Pulumi request before any session token is exposed.

7. A continuation token is bound to the original Pulumi `token`, normalized
   query, and `limit`. A continuation request that changes any of those fields
   fails with `InvalidArgument`.

8. `page_size` bounds the number of result messages in a single Pulumi `List`
   RPC response. A provider may return fewer. It must not return more.

9. `limit` bounds the total number of results across all continuation requests.
   `limit < 1` means no Pulumi limit. The bridge may pass an implementation
   limit to Terraform, but must enforce Pulumi `limit` itself and keep provider
   buffering bounded.

10. Results are streamed as zero or more `ListResponse.Result` messages
    followed by exactly one `ListResponse.Continuation` marker. The final
    marker has an empty token.

11. Terminal Terraform diagnostics, conversion failures, or ID extraction
    failures are not silently swallowed. If an error occurs after some results
    have been produced, the current Pulumi RPC returns the error rather than a
    successful empty continuation.

12. The bridge should request resource-object results from Terraform by setting
    `IncludeResource: true`.

13. `ListResponse.Result.Id` is a best-effort string handle for the listed
    resource. Terraform does not expose a generic string import-ID format for
    structured identity data, so the bridge should fail open with deterministic
    Pulumi-owned formatting instead of hiding otherwise useful list results.
    Prefer an explicit `id` when Terraform returns one. If no `id` is
    available, derive an ID from Terraform identity data as described in "Result
    IDs" below.

14. Muxed providers use list-specific dispatch. A Pulumi resource's `List` RPC
    routes to the subprovider that owns the Terraform list resource and list
    config schema, not necessarily the subprovider that owns CRUD for that
    managed resource.

15. Plain SDKv2 `helper/schema` resources remain unlistable unless an upstream
    provider exposes a Terraform protocol list resource for them, for example
    through a framework wrapper in a muxed provider.

16. Session cleanup cancels the Terraform list context, marks waiting sessions
    done, broadcasts waiters, and removes the token. Provider shutdown, session
    expiration, RPC cancellation, and send failure must not leave waiters
    blocked indefinitely.

## Validation Boundary

Build time:

- Discover Terraform list resources from schema-only provider shims and muxed
  provider subproviders.
- Generate `listInputs` from Terraform list config schemas.
- Ensure providers without list resources produce empty list-resource maps, not
  nil dereferences.
- Preserve list-resource ownership separately from CRUD ownership in muxed
  providers.

Runtime startup:

- Initialize the list session store for list-capable providers.
- Do not require list schemas to exist for providers that never receive a
  `List` request.
- Initialize muxed list dispatch from the same generated metadata source used
  for resource/function dispatch, extended with list-resource ownership.

Per `List` request:

- Reject invalid request bounds.
- Resolve Pulumi token to the Terraform list resource owner and list schema.
- Detect unknown/computed query values before Terraform calls.
- Validate concrete list config with Terraform.
- Start and track a provider-owned session for first pages.
- Validate continuation request identity for continuation pages.
- Stream result and continuation messages in protocol order.
- Propagate terminal errors.

## Design

### Schema Discovery

Make `GatherListResources` return an empty `listResources` collection when the
PF provider does not implement `ProviderWithListResources`. Alternatively make
`newSchemaOnlyListResourceMap` nil-safe, but the preferred ownership is to keep
the gatherer result non-nil whenever discovery succeeded.

Keep list resource schemas separate from managed resource schemas. Do not infer
listability from managed resource inputs, data sources, or Terraform naming
alone.

For muxed providers, carry list-resource maps through the mux shim in addition
to managed resource and data source maps. If a list resource and managed
resource share a Terraform type but come from different subproviders, schema
generation must use the list-resource schema from the list owner and the
managed resource schema from the CRUD owner.

For protocol-v5 providers, list discovery is possible only when the upstream
provider server exposes Terraform protocol list resources. SDKv2
`helper/schema` by itself is not enough; terraform-provider-aws works because
its framework provider exposes `@SDKListResource` wrappers through
`ProviderWithListResources`.

### Query Conversion

Replace direct `query.AsMap()` conversion with the same Pulumi property
unmarshal model used by other RPCs, with `KeepUnknowns` enabled. Walk the
resulting property map for computed values before converting to Terraform.

If any query value is computed, send:

```text
ListResponse{Computed: {}}
```

and return without creating a session.

For concrete values, encode the query into the Terraform list config schema
type. Conversion failures are `InvalidArgument`.

### Terraform Validation

Before storing the session token, call:

```text
ValidateListResourceConfig(TypeName, Config)
```

Process diagnostics using the existing PF diagnostic path. Error diagnostics
fail the Pulumi request and no background list goroutine is started.

When calling Terraform `ListResource`, request resource-object results with
`IncludeResource: true`. Pulumi needs an importable result ID, and
resource-object results may contain a provider-populated top-level `id`.
Providers may still return identity-only results; those remain governed by the
identity policy below.

### Session Identity

Extend `listSession` with immutable request identity:

- Pulumi token
- normalized query fingerprint
- limit

The query fingerprint should be derived after Pulumi unmarshalling and before
Terraform dynamic encoding so equivalent Pulumi query values compare stably.
Continuation calls must compare against that identity before acquiring the
session.

### Result IDs

Prefer Terraform resource-object results because they can contain the provider's
normal state shape. If the decoded resource object has a concrete top-level
`id` value, stringify that value as `ListResponse.Result.Id`. Do not call
`extractID` or `ResourceInfo.ComputeID` for list results; those paths produce
Pulumi state IDs and are not proof of Terraform string import-ID compatibility.

If the resource object has no usable top-level `id`, or if Terraform returns an
identity-only result, decode the Terraform identity data with the Terraform
resource identity schema and derive a Pulumi-owned string ID:

1. If there is an identity attribute named `id`, stringify that value.
2. If there is exactly one identity attribute, stringify that attribute value.
3. If there are multiple identity attributes, sort them by attribute name,
   stringify each value, and join the values with `,`.

This default format is intentionally a Pulumi list result format, not a claim
that Terraform import accepts the same string. Terraform does not expose the
provider's string import-ID formatter for structured identities. The derived ID
keeps users unblocked and gives them the listed identity values; users may still
need to adjust the string manually when importing through existing `Read`.
Missing identity schemas, identity decode failures, and values that cannot be
stringified deterministically should fail with an error that names the Terraform
resource and explains that no list ID could be derived.

### Muxed Providers

Add list-specific mux dispatch. The current muxer dispatches resource CRUD by
Pulumi resource token and function calls by function token. `List` also uses a
resource token, but its owner can differ from the CRUD owner. The dispatch
table therefore needs to record which subprovider owns list support for each
Pulumi resource token.

Runtime `List` handling in the muxer should route to that list owner. If no
subprovider exposes a list resource for the token, return `Unimplemented`. Do
not route `List` to a CRUD owner merely because it serves the managed resource.

Generated schema merging must preserve `listInputs` from the list owner. If
the managed resource owner and list owner disagree about the resource token,
the generated Pulumi schema should still attach the list inputs to the final
Pulumi resource spec for that token.

### Error And Cancellation Handling

Treat list sessions as owned by the provider runtime, not by a single RPC.
Closing a session should:

- cancel the Terraform list context
- set `done`
- store a cancellation or expiration error when appropriate
- broadcast the condition variable

`ListWithContext` should return terminal errors even if the page already
contains results. It must not convert a terminal error into an empty successful
continuation.

## Compatibility Risks

- **Generated schema stability:** Adding `listInputs` changes provider package
  schemas for providers with list resources. Providers without list resources
  must have no generated schema drift.
- **SDKv2/PF parity:** Plain SDKv2 `helper/schema` resources remain unlistable,
  but SDKv2-managed resources exposed through framework list wrappers in muxed
  providers are listable. The docs and errors need to preserve that
  distinction.
- **Muxed providers:** A muxed provider may combine SDKv2 CRUD resources and PF
  list resources for the same Terraform type. `List` must dispatch by list
  ownership, not CRUD ownership.
- **Dynamic bridge:** Dynamic providers consuming generated schema need stable
  `listInputs` output. Add dynamic/golden coverage before changing dynamic
  schema behavior.
- **Imports and refresh:** `ListResponse.Result.Id` is best-effort for
  structured identities because Terraform does not expose a generic string
  import-ID format. IDs derived from identity data may need manual adjustment
  before use with the existing `Read` import path.
- **Secrets and unknowns:** Query conversion must preserve Pulumi unknowns and
  should reject or unwrap secrets consistently with other request inputs.
- **Diagnostics timing:** Terraform validation diagnostics should fail before a
  session token exists; list-stream diagnostics should fail the active RPC and
  clean up the session.
- **Resource consumption:** Terraform list APIs are streaming, while Pulumi
  exposes page-oriented continuations. Provider-owned sessions may prefetch.
  The implementation must enforce Pulumi `page_size` and `limit` even when
  Terraform produces more results, and it must avoid unbounded buffering when
  Pulumi `limit` is unset.

## Test Plan

Build-time schema tests:

- `pkg/pf/internal/schemashim`: provider without `ProviderWithListResources`
  returns an empty list map and does not panic.
- `pkg/pf/tfgen`: list resource with required and optional config fields emits
  cased `listInputs`.
- `pkg/pf/tfgen`: empty list config schema emits empty non-nil `listInputs`.
- `pkg/pf/tfgen`: no list resource emits nil `listInputs`.
- `pkg/pf/tfgen`: list resource without a matching managed resource is ignored
  or fails with a documented error.
- PF mux/tfgen: muxed resource whose CRUD owner and list owner differ emits
  `listInputs` from the list owner on the final Pulumi resource spec.

Runtime tests:

- First-call `List` path covers token mapping, query conversion,
  `ValidateListResourceConfig`, `ListResource`, result streaming, and final
  continuation.
- Unknown/computed query returns one `Computed` response and does not call
  Terraform.
- Invalid concrete query returns `InvalidArgument`.
- Terraform validation diagnostics fail before session creation.
- Changed continuation `token`, `query`, or `limit` is rejected.
- `page_size` and `limit` combinations: `limit < page_size`, exact boundary,
  multi-page limit exhaustion, and provider overproduction.
- Resource-object result with a top-level `id` uses that value without calling
  `extractID` or `ComputeID`.
- Identity result with one attribute stringifies that value.
- Identity result with multiple attributes uses the documented sorted
  comma-joined Pulumi ID format.
- Terraform `ListResource` is called with `IncludeResource: true`.
- Terminal error after partial results returns an error, not a successful empty
  continuation.
- Session close/expiration wakes `preparePage` waiters.
- Send failure cancels or removes the session consistently.

Provider server tests:

- `pkg/pf/internal/plugin`: gRPC `List` forwarding preserves stream behavior.
- `pkg/providerserver`: panic recovery wraps streaming `List` and logs method
  context.

Integration and compatibility tests:

- `pkg/pf/tests`: ordinary PF providers without list resources continue to
  construct and pass existing tests.
- `pkg/x/muxer` or PF mux tests: SDKv2-only resources without a Terraform list
  resource return `Unimplemented`; muxed SDK-backed list resources dispatch to
  the PF/framework list owner.
- `pkg/x/muxer` or PF mux tests: CRUD dispatch and list dispatch may target
  different subproviders for the same Pulumi resource token.
- Dynamic bridge schema/golden tests if generated schema output for dynamic
  providers changes.

Suggested focused commands:

```bash
mise exec -- go test -count=1 ./pkg/pf/internal/schemashim ./pkg/pf/tfgen ./pkg/pf/tfbridge -run 'Test.*List|Test.*ListResource'
mise exec -- go test -count=1 ./pkg/pf/tests -run TestCallWithTerraformConfig
mise exec -- go test -count=1 ./pkg/providerserver -run TestPanicRecoveringProviderServer_List
mise exec -- make test_assets
mise exec -- make lint
```
