# GH-3430 Default Token Fixup Laziness

## Summary

Issue [#3430](https://github.com/pulumi/pulumi-terraform-bridge/issues/3430)
tracks a remaining broad-schema-load path after lazy Terraform Plugin Framework
schema loading. In muxed SDKv2/PF providers, provider startup can still load
every PF resource schema while computing tokens and default compatibility
fixups, even when the Pulumi program uses only SDKv2 resources.

The desired behavior is:

1. Token/resource metadata initialization may enumerate resource names.
2. Startup must not call `Schema()` for every PF resource.
3. Existing default compatibility behavior for `id`, missing IDs, `urn`, and
   `pulumi` remains available for resources that need it.
4. A selected PF resource may load its own schema as part of the normal PF RPC
   path; unused PF resources must remain unloaded.

This is a runtime memory behavior change. It should not change generated
Pulumi schema output.

## Current Flow

Downstream providers commonly call `ProviderInfo.MustComputeTokens` during
provider initialization. That enters `pkg/tfbridge/tokens.ComputeTokens`, which
calls `applyDefaultFixups` before default resource and data source token
computation.

The broad load is in the default fixup pass:

1. `ComputeTokens` calls `applyDefaultFixups` unless `SkipDefaultFixups` is set.
2. `applyDefaultFixups` calls `fixPropertyConflict` and `fixMissingIDs`.
3. Both fixups call `walkResources`.
4. `walkResources` enumerates `p.P.ResourcesMap()`.
5. `fixPropertyConflict` calls `r.TF.Schema().Range(...)`.
6. `fixMissingIDs` calls `r.TF.Schema().GetOk("id")`.

On a PF schema-only resource, `schemaOnlyResource.Schema()` delegates to the
lazy PF schema adapter. Calling it forces the selected resource schema to load.
Because the current fixup pass walks every resource, a muxed provider pays for
all PF resource schemas during startup.

Relevant code:

- `pkg/tfbridge/tokens/tokens.go`: `ComputeTokens`.
- `pkg/tfbridge/tokens/fixups.go`: `applyDefaultFixups`,
  `fixPropertyConflict`, `fixMissingIDs`, and `walkResources`.
- `pkg/pf/internal/schemashim/resource.go`: `schemaOnlyResource.Schema`.
- `pkg/pf/internal/pfutils/lazy_schema.go`: lazy schema load triggers such as
  `Attrs`, `Type`, and `ResourceProtoSchema`.
- `pkg/pf/internal/muxer/muxer.go`: PF ownership helpers such as
  `ResourceIsPF`.

## Desired Semantics

1. Default token computation still creates missing `Resources` and
   `DataSources` entries from resource and data source names.
2. Build-time schema-generation paths still run schema-dependent default fixups
   for PF-owned resources, because those paths already materialize PF schemas.
3. Runtime startup for generated/static PF-owned resources consumes precomputed
   fixup metadata and does not inspect every PF resource schema.
4. SDKv2 resources keep the current eager default fixup behavior.
5. Startup fixups remain idempotent. If provider code already set a field
   rename or `ComputeID`, runtime metadata application must preserve it.
6. `IgnoreMappings` still skips ignored resources.
7. `SkipDefaultFixups` still disables default fixups globally.
8. Provider-resource token fixup remains name-based and should not require PF
   resource schema loading.
9. `GetMapping` must continue to expose the same field-name compatibility
   decisions for conversion metadata. A solution that only mutates
   `ResourceInfo` after a selected runtime RPC is not sufficient unless it first
   proves those deferred decisions are irrelevant to mapping consumers.
10. Because `ComputeTokens` is used by both tfgen/build-time and runtime
    provider startup, any skip capability must be gated by phase or by proven
    presence of equivalent precomputed metadata. A blind
    `DeferResourceSchemaFixups` hook would risk changing generated schema
    output.

## Design

Split default fixups into name-only work, schema-derived field metadata, and
runtime-only `ComputeID` behavior.

The name-only token path should continue to enumerate `ResourcesMap` and
`DataSourcesMap` so bridge metadata and dispatch can be built. Enumeration is
cheap for PF schema-only resources because the lazy PF branch already gathers
type metadata without calling resource `Schema`.

The schema-dependent resource fixup path should become reusable for one
resource at a time. The existing `fixID`, `fixURN`, `fixPulumi`, and missing-ID
logic are already mostly per-resource; the implementation should factor them
behind a helper that accepts `resourceInfo` and applies the same compatibility
rules to one resource. Keep the field-name decisions (`SchemaInfo.Name` and
`SchemaInfo.Type`) separable from runtime-only `ComputeID` functions, because
`MarshalProviderInfo` persists fields into mapping data but does not persist
`ComputeID`.

The first-choice implementation is build-time precomputation:

1. During tfgen/schema generation, keep running schema-dependent default fixups
   for PF-owned resources. This preserves generated Pulumi schema output and
   mapping field metadata while schemas are already allowed to load.
2. Persist or preserve the resulting field decisions in generated/provider
   metadata. At minimum this covers `SchemaInfo.Name` and `SchemaInfo.Type` for
   `id`, `urn`, and `pulumi` compatibility.
3. Store compact runtime fixup metadata in `ProviderInfo.MetadataInfo`
   (`bridge-metadata.json`) under a new bridge-owned key, and declare that key
   as runtime metadata so `ExtractRuntimeMetadata` copies it into
   `runtime-bridge-metadata.json` when providers generate a runtime-only
   metadata file. The metadata should describe the default `ComputeID` decision
   without embedding functions, for example:
   - missing ID placeholder;
   - delegate ID from a concrete Pulumi field name;
   - no default runtime `ComputeID` needed.
   `ExtractRuntimeMetadata` should also write a bridge-internal runtime marker
   into the generated metadata blob. Runtime providers should keep using the
   existing `tfbridge.NewProviderMetadata(...)` call shape; the shared
   `ComputeTokens` path should detect runtime metadata from the marker, not from
   provider call-site changes.
4. During runtime startup, call a single helper such as
   `applyPrecomputedDefaultFixups(&info)` immediately after embedded metadata is
   available and before constructing the RPC provider object or mux mapping
   closures. The helper mutates the runtime `ProviderInfo` copy once, before
   `GetMapping`, encoder/decoder setup, defaults, diagnostics, or lifecycle
   paths can observe `ResourceInfo`.

Only after proving the precomputed metadata is present should runtime startup
skip schema-dependent live schema inspection for PF-owned resources. Keep PF
ownership as a narrow signal instead of importing PF internals into
`pkg/tfbridge/tokens`. This interface is only one input to the runtime decision,
not the decision itself:

```go
type precomputedResourceSchemaFixupCandidate interface {
    ResourceSchemaFixupsMayBePrecomputed(tfToken string) bool
}
```

The PF schema-only provider can return true for its resources. The mux provider
can return true only for resources owned by the PF side, preserving SDKv2 eager
fixups. The existing mux ownership helper `ResourceIsPF` is the current model
to mirror. The runtime skip predicate should be closer to:

```text
runtime mode
&& resource is a precomputed-fixup candidate
&& metadata has a resource entry for this tfToken
```

If any part is false, runtime must keep the existing schema-inspecting behavior
or fail explicitly rather than silently dropping fixups.

An empty resource entry is meaningful: it means build time inspected that
resource schema and found no default field rename or `ComputeID` fixup to
replay. A missing resource entry is different and should not be treated as
proof that no fixups are required.

The implementation must distinguish build-time/schema-generation from runtime
startup. Either add an explicit phase or mode to the shared token/fixup path, or
make the runtime skip conditional on finding equivalent precomputed metadata for
the PF-owned resource. Without that guard, the same `ComputeTokens` change could
silently skip schema-derived fixups during tfgen and change generated schema
output.

Selected-resource runtime fixups are a fallback, not the primary design. Use
them only if the build-time metadata route cannot represent a specific
runtime-only decision, and only after proving late mutation cannot affect
`GetMapping`, encoders/decoders, defaults, diagnostics, schema metadata, or
other code that expects final `ResourceInfo.Fields`.

Keep the fixup helper idempotent. The existing rules already preserve explicit
provider decisions such as field-name overrides and custom `ComputeID`; tests
should keep those protections.

## Compatibility Risks

- `id` handling is the highest-risk behavior. The current eager fixup can rename
  input IDs, preserve computed string IDs, synthesize `ComputeID`, and install a
  missing-ID placeholder. Precomputed metadata must cover the same cases without
  broad runtime PF schema inspection.
- `urn` and `pulumi` are narrower because they only need to know whether those
  top-level Terraform properties exist and whether the proposed replacement
  name is available.
- Direct PF providers and muxed PF providers should share the same runtime
  metadata-consumption behavior. SDKv2-only providers should not change.
- Build-time schema generation still needs full schema materialization and
  must not use the runtime skip path.
- `ComputeTokens` is shared by build-time and runtime startup. Any phase/mode
  split must be explicit enough that tfgen cannot accidentally skip
  schema-derived fixups.
- Late selected-resource mutation is risky because mapping data, encoders,
  decoders, defaults, diagnostics, and schema metadata may already have observed
  `ResourceInfo`.
- Dynamic providers or parameterized providers may not have the same static
  generated-schema assumptions. If a provider cannot safely defer, it should not
  opt into the deferred-fixup capability.

## Validation

Add focused bridge tests before downstream AWS proof.

1. Unit-test the factored per-resource fixup helper in
   `pkg/tfbridge/tokens/fixups_test.go` by reusing the existing `id`, missing
   ID, `urn`, `pulumi`, ignored mapping, and explicit override cases.
2. Add build-time/schema-generation coverage proving PF schemas still receive
   schema-dependent default fixups and generated field metadata does not change
   for `id`, `urn`, and `pulumi` cases.
3. Add metadata coverage proving the new fixup metadata is written to
   `bridge-metadata.json`, included by `ExtractRuntimeMetadata` in
   `runtime-bridge-metadata.json`, reloaded through the existing
   `NewProviderMetadata(runtimeMetadata)` call shape, and applied before both
   SDKv2 and PF `GetMapping` marshal `ProviderInfo`.
4. Add runtime metadata-consumption coverage proving `ComputeTokens` can skip
   live PF schema inspection only when equivalent precomputed metadata is
   present.
5. Add a startup regression in `pkg/pf/tests/mux_lazy_schema_test.go`:
   construct a muxed SDKv2/PF provider, call `ComputeTokens` or the same
   provider-info initialization path used by the test provider, and assert the
   PF counting provider still reports zero resource schema calls.
6. Extend the same mux test to select one PF resource and assert only that PF
   resource schema is loaded, not unrelated PF resources.
7. Add PF metadata cases for:
   - required or optional/computed `id` needing rename and delegated ID;
   - no `id` needing missing-ID behavior;
   - top-level `urn` rename;
   - top-level `pulumi` rename.
8. Run targeted tests first:

```bash
make test RUN_TEST_CMD='./pkg/tfbridge/tokens -run Test'
make test RUN_TEST_CMD='./pkg/pf/tests -run TestMuxedSDKv2OperationsDoNotLoadPFResourceSchemas'
```

9. For final confidence, rerun the downstream AWS SDKv2-only S3 memory probe
   that motivated the issue and verify zero broad PF schema loads during
   startup.
