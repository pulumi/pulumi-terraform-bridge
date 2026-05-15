# GH-3428 Lazy PF Schema Runtime Semantics

## Summary

Issue [#3428](https://github.com/pulumi/pulumi-terraform-bridge/issues/3428) tracks a memory cost in muxed providers that include Terraform Plugin Framework resources or data sources. Runtime startup currently materializes the full PF provider schema in the long-lived Pulumi provider process, even when a Pulumi program uses only SDKv2 resources or only a small subset of PF resources.

Local experiments on a large muxed provider showed provider-process RSS reductions of roughly 120-180 MiB when runtime startup avoided eager full-provider PF schema materialization. The target model is:

1. Build-time schema generation still materializes and validates the full provider schema.
2. Runtime startup gathers cheap PF metadata needed for dispatch and type-name resolution.
3. Runtime startup does not call full PF `GetProviderSchema`.
4. Runtime operations lazily materialize only the PF resource or data source schemas they use.
5. SDKv2-only programs in muxed providers do not pay the full PF schema cost.
6. Generated Pulumi schema and bridge metadata remain unchanged.

This is a runtime memory change, not a schema-generation behavior change.

## Current Flow

### Build-Time Schema Generation

PF schema generation enters through `pkg/pf/tfgen`.

- `pkg/pf/tfgen/gen.go` calls `check.Provider` before delegating to the standard `pkg/tfgen` generator.
- `pkg/tfgen` walks `info.P.Schema()`, `info.P.ResourcesMap()`, and `info.P.DataSourcesMap()` to build the Pulumi package schema.
- For PF providers, `info.P` is a schema-only shim built by `pkg/pf/internal/schemashim`.

With the current lazy-schema experiment, tfgen still materializes resource and data source schemas because schema generation needs complete Pulumi package schema output.

One important gap: the bridge currently runs Pulumi PF checks during tfgen, but a canary test showed that `pkg/pf/tfgen.GenerateSchema` does not currently fail on a Framework schema implementation error that `ValidateImplementation` would reject. Full Framework implementation validation is therefore currently obtained in the runtime path as a side effect of full `GetProviderSchema`, not as an explicit build-time check.

### Runtime Startup

PF runtime providers are constructed in `pkg/pf/tfbridge/provider.go`.

1. `newProviderWithContext` asserts `info.P` implements `pf.ShimProvider`.
2. It calls `pfServer.Server(ctx)` to create a Terraform protocol v6 server.
3. It calls `pfServer.Resources(ctx)` and `pfServer.DataSources(ctx)` to get runtypes collections.
4. It calls `pfServer.Config(ctx)` to get provider config type information.
5. It creates config/resource/data-source encoders later from the selected schema types.

`SchemaOnlyProvider.Server` currently creates a Framework protocol server with `providerserver.NewProtocol6(p.tf)` and calls `GetProviderSchema` once. The comment says this was needed to avoid "Resource Type Not Found" from incorrect provider-name initialization, such as `random_integer` becoming `_integer`.

That full `GetProviderSchema` call is the expensive part. In Terraform Plugin Framework, `GetProviderSchema` walks provider schema, provider meta schema, every resource schema, every data source schema, functions, ephemeral resources, list resources, actions, and state stores. It also validates schema implementation while doing so.

### PF Metadata and Per-Resource Schema Loading

Current Terraform Plugin Framework versions expose a cheaper metadata path:

- `GetMetadata` enumerates resource and data source type names and server capabilities.
- Per-resource server operations can later call `ResourceSchema(typeName)`, which loads and caches only the selected resource schema.
- Per-data-source operations similarly load only the selected data source schema.

The bridge already has enough local metadata gathering to build resource and data source maps without calling `resource.Schema()`:

- `GatherResources` calls provider `Metadata`, provider `Resources`, and each resource `Metadata`.
- `GatherDatasources` calls provider `Metadata`, provider `DataSources`, and each data source `Metadata`.

The branch experiment changes those gatherers so each map entry stores a lazy schema adapter. Methods such as `Type`, `Attrs`, `Blocks`, `Shim`, `ResourceSchemaVersion`, and `ResourceProtoSchema` force the real schema only when used.

### Runtime Resource Operations

PF runtime operations usually load the selected resource schema at `resourceHandle` construction:

- Resolve Pulumi token to Terraform type name.
- Fetch `p.resources.Schema(typeName)`.
- Use `schema.TFName()` for the real Terraform type.
- Call `schema.Type(ctx)` to create resource encoders and decoders.

After that, Check, Diff, Create, Update, Delete, Read, import, state upgrade, raw-state delta, and detailed diff may use the same schema for type encoding, Pulumi default handling, diagnostics, `ResourceSchemaVersion`, and raw-state computation.

This means lazy mode does not avoid schema cost for a used PF resource. It avoids loading every unused PF resource.

### Runtime Data Source Operations

PF Invoke builds a `datasourceHandle`:

- Resolve Pulumi token to Terraform data source type name.
- Fetch `p.datasources.Schema(typeName)`.
- Call `schema.Type(ctx)` to create encoders and decoders.
- Call PF `ValidateDataResourceConfig` and `ReadDataSource`.

Lazy mode should load only the invoked data source schema.

### Muxed Providers

Muxed providers combine SDKv2 and PF providers.

- `pkg/pf/internal/muxer` extends a base shim with a PF schema-only provider.
- Dispatch metadata chooses the sub-provider for each Pulumi resource or function.
- SDKv2 should dominate conflicts.

The critical GH-3428 case is a muxed provider where the user program touches only SDKv2 resources. Runtime must still know PF resource/data source type names for dispatch metadata and aliases, but it should not materialize all PF schemas.

## Desired Semantics

1. `tfgen` and build-time helper binaries fully materialize the provider schema and generate the same Pulumi package schema as before.
2. Build-time should run bridge PF validation and Framework schema implementation validation so invalid provider schemas fail before release.
3. Runtime startup should not call PF full `GetProviderSchema` for static generated providers.
4. Runtime startup may call metadata-only APIs or local provider/resource/data source `Metadata` methods to establish type-name maps.
5. Runtime provider config schema may remain eager because provider configuration is needed for `CheckConfig`, `Configure`, and config encoding.
6. PF resource schemas should be loaded only when a PF resource operation actually needs that resource.
7. PF data source schemas should be loaded only when a PF invoke actually needs that data source.
8. SDKv2-only operations in muxed providers should not call PF resource/data source `Schema` methods.
9. Mux dispatch, alias resolution, conflict detection, and resource/data source map enumeration should not force PF schemas.
10. Existing PF lifecycle behavior must remain unchanged for used PF resources and data sources.
11. Generated schema, bridge metadata, and dispatch metadata should remain unchanged.
12. Dynamic bridge schema compatibility must not drift unless explicitly accepted.

## Validation Boundary

### Build-Time Validation

Build time should validate provider schema implementation because Pulumi static providers already separate schema generation from runtime execution.

Required build-time checks:

- Existing bridge PF checks in `pkg/pf/internal/check`.
- Framework provider schema implementation validation.
- Framework resource schema implementation validation for every generated PF resource.
- Framework data source schema implementation validation for every generated PF data source.

If Framework validation fails during a provider upgrade, the actionable outcomes are pinning, patching, reporting upstream, or waiting for a fixed upstream release. Runtime cannot repair an invalid upstream provider schema, so runtime should not pay full provider-wide validation cost on every provider process startup.

This is a static generated-provider guarantee. Build-time validation should catch invalid schemas for the released provider binary, but runtime lazy loading can still fail if schema construction depends on runtime-only environment, parameterization, dynamic-provider discovery, or code paths that differ from tfgen.

### Runtime Startup Validation

Runtime startup should validate only what it needs to start safely:

- provider metadata and type-name availability;
- provider config schema/type information;
- bridge metadata presence;
- mux dispatch table presence for muxed providers.

Runtime startup should not validate every PF resource/data source schema.

### Runtime User-Input Validation

Runtime must still run user-input validations because these depend on the Pulumi program:

- `CheckConfig` and PF `ValidateProviderConfig`;
- `Check` and PF `ValidateResourceConfig`;
- `Invoke` and PF `ValidateDataResourceConfig`;
- diagnostics processing and CheckFailure conversion.

These are not replaceable by tfgen because they validate actual config, inputs, unknowns, secrets, defaults, and provider-specific diagnostics.

## Design

### Build-Time Framework Validation

Add explicit Framework implementation validation to the PF build-time path.

Possible location:

- `pkg/pf/internal/check` if the validation should be part of `check.Provider`;
- or a new helper under `pkg/pf/internal/schemavalidation` if direct Framework schema validation would make `check` too broad.

The helper should:

1. Detect PF-backed providers and PF-backed entries in muxed providers.
2. Validate provider schema.
3. Iterate PF resources that are included in generation and validate each resource schema.
4. Iterate PF data sources that are included in generation and validate each data source schema.
5. Convert Framework diagnostics into bridge errors with resource/data source names.

Implementation constraint: Framework `ValidateImplementation` methods operate on the concrete Framework schema values (`provider/schema.Schema`, `resource/schema.Schema`, and `datasource/schema.Schema`). The existing `tfbridge.ProviderInfo.P` surface is a shim provider, so this validation either needs to happen while those concrete schema values are still available or needs a narrow internal hook from `schemashim`/`pfutils` that exposes validation without broadening the public shim interfaces.

The canary test should prove `pkg/pf/tfgen.GenerateSchema` fails for a PF schema that Framework `ValidateImplementation` rejects, such as an optional string attribute with a default but without `Computed`.

### Lazy Bridge Schema Adapters

Keep the lazy adapter shape from the experiment:

- gather metadata eagerly;
- store `tfName` eagerly;
- store a `sync.Once` loader for the real PF schema;
- materialize on calls to `Type`, `Attrs`, `Blocks`, `Shim`, `ResourceSchemaVersion`, `DeprecationMessage`, and `ResourceProtoSchema`.

The current `runtypes.Schema` interface does not return errors from those methods, so schema-load failures either need to panic or require a broader interface change. For this issue, panic-on-load-error is acceptable only if build-time validation is already responsible for catching invalid provider schemas before release. Runtime panics from schema loading should still be wrapped by the provider server panic recovery path.

Experimental env vars and tracing should not be part of the final production surface. If load tracing is useful, keep it test-only or use existing logging conventions.

### Runtime Server Initialization

Remove the full `GetProviderSchema` call from `SchemaOnlyProvider.Server` after proving type-name resolution still works.

Prefer no protocol initialization call if direct local metadata gathering is sufficient. If a narrow protocol call is still needed, prefer `GetMetadata` over `GetProviderSchema` because it exercises provider/resource/data source metadata without full schema construction. Static runtime startup should not use full `GetProviderSchema`.

The implementation should include a regression test for the old provider-name problem:

- define a PF provider whose resource `Metadata` uses `req.ProviderTypeName + "_thing"`;
- create the runtime server without full `GetProviderSchema`;
- call a resource RPC such as `ValidateResourceConfig` or `PlanResourceChange`;
- assert the server recognizes `test_thing`, not `_thing`.

### Muxed Provider Behavior

For muxed providers:

- PF metadata collection may happen during augmentation.
- Dispatch resolution must not call PF resource/data source `Schema`.
- SDKv2-only operations must route to the SDKv2 provider without loading PF schemas.
- PF resource/data source operations may load the selected PF schema and only that schema.

### Generated Schema Stability

Because tfgen still materializes full schema, lazy runtime should not change:

- package schema JSON;
- bridge metadata;
- dispatch table;
- generated SDKs;
- docs output.

Any generated diff from this work should be treated as suspicious unless the implementation intentionally changes build-time validation errors only.

## Compatibility Risks

### Framework Validation Timing

Dropping runtime `GetProviderSchema` removes provider-wide Framework schema implementation validation from runtime startup. This is acceptable only if equivalent validation is added to tfgen/build time for static providers.

### Provider Name Initialization

The existing comment says `GetProviderSchema` was needed to avoid "Resource Type Not Found" caused by missing provider type names. Current Framework code has metadata-only paths, but this needs a regression test.

### Lazy Error Timing

Errors from a bad resource/data source schema move from provider shim construction or startup to first schema use. Build-time validation should prevent this in released static providers, but tests should pin the expected behavior.

### SDKv2/PF Parity

SDKv2 schema behavior should not be changed by PF lazy schema work. Any SDKv2 laziness changes should be separate unless directly required.

### Dynamic Bridge

Dynamic providers may depend on full runtime provider schema discovery. The lazy static-provider path should not silently change dynamic schema generation or dynamic provider runtime behavior.

### Mux Dispatch and Aliases

Alias insertion and dispatch resolution use resource/data source maps. Tests must prove those operations remain metadata-only and do not force PF schema loading.

### State Upgrade and Raw State

PF state parsing, `UpgradeResourceState`, `ResourceSchemaVersion`, `RawStateDelta`, and private state all use the selected resource schema. Lazy loading is fine for a used resource, but tests need to cover these paths.

### Diagnostics Timing

Framework schema implementation diagnostics currently appear during runtime startup because of `GetProviderSchema`. After this change they should appear during tfgen/build time. User-input diagnostics must continue to appear during runtime operations.

## Rejected Shortcuts

### Keep Full Runtime `GetProviderSchema`

This preserves current validation and initialization behavior, but it keeps the memory problem. It is not a solution to GH-3428.

### Only Add Lazy Wrappers

Lazy bridge wrappers alone are insufficient if runtime startup still calls full PF `GetProviderSchema`, because that call materializes every PF resource and data source schema.

### Drop Validation Without Moving It

Skipping runtime `GetProviderSchema` without adding build-time Framework validation weakens provider release checks. Runtime cannot fix invalid upstream schema implementation, but the build should catch it.

### Use Generated Pulumi Schema For All Runtime Encoding

Generated Pulumi schema is not a complete replacement for PF runtime schema because PF protocol calls need Terraform types, schema versions, and conversion details for the selected resource or data source.

### Treat Dynamic Bridge Like Static Generated Providers

Dynamic providers have different schema discovery constraints. Do not apply static-provider assumptions to dynamic bridge without a dedicated compatibility check.

## Test Plan

### Build-Time Tests

1. **Framework validation canary**
   - Add a PF resource schema that Framework `ValidateImplementation` rejects.
   - Run `pkg/pf/tfgen.GenerateSchema`.
   - Assert generation fails with a diagnostic identifying the invalid resource schema.

2. **Provider config schema validation**
   - Add a provider schema invalid under Framework implementation validation.
   - Assert tfgen fails before package schema output.

3. **Data source schema validation**
   - Add an invalid PF data source schema.
   - Assert tfgen fails and identifies the data source.

4. **Valid schema still generates**
   - Existing PF tfgen tests continue to pass.

### Lazy Schema Unit Tests

1. Construct a PF schema-only provider with multiple counted resources and data sources.
2. Assert `ShimSchemaOnlyProvider` calls provider/resource/data source metadata but not resource/data source `Schema`.
3. Assert `ResourcesMap().Len`, `Range`, and `GetOk` do not load schemas.
4. Assert accessing one resource schema loads exactly that resource once.
5. Assert accessing one data source schema loads exactly that data source once.
6. Assert concurrent access to the same schema loads once.

### Runtime Startup Tests

1. Start a PF provider server without full `GetProviderSchema`.
2. Assert provider config type and config validation still work.
3. Assert resource type-name resolution works for a resource whose metadata depends on provider type name.
4. Assert data source type-name resolution works similarly.

### PF Runtime Lifecycle Tests

Run targeted tests for:

- Check and `ValidateResourceConfig`;
- Diff and detailed diff;
- Create preview and apply;
- Update preview and apply;
- Delete;
- Read refresh;
- import;
- state upgrade;
- raw state delta;
- Invoke and `ValidateDataResourceConfig`;
- provider `CheckConfig` and `Configure`.

### Mux Tests

1. Mux SDKv2 and PF providers.
2. Run an SDKv2-only operation and assert no PF resource/data source schema loads.
3. Run one PF resource operation and assert only that resource schema loads.
4. Run one PF data source invoke and assert only that data source schema loads.
5. Assert alias and dispatch resolution do not force PF schema loading.

### Generated Output Checks

For representative providers:

- package schema JSON is unchanged;
- bridge metadata is unchanged;
- dispatch table is unchanged;
- dynamic bridge golden output is unchanged where relevant.

### Downstream Memory Proof

Use the large muxed provider that motivated GH-3428 and measure provider-process RSS for:

1. SDKv2-only program;
2. one PF resource;
3. one PF data source;
4. multiple PF resources;
5. destroy paths for the same cases.

Compare eager baseline versus lazy runtime. The expected result is that SDKv2-only programs avoid PF full-schema cost, and PF programs pay only for used PF schemas instead of the full PF surface.

## Open Decisions

1. Whether lazy schema load failures should continue to panic or whether `runtypes.Schema` should grow error-returning accessors. For GH-3428, avoid broad interface churn unless tests show panic timing is unacceptable.
2. Whether runtime should call `GetMetadata` during server initialization or rely on direct local metadata plus the Framework server's per-RPC metadata paths. Prefer no protocol call if that proves provider-name initialization correctness; otherwise use `GetMetadata`, not `GetProviderSchema`.
3. Whether Framework validation belongs inside `pkg/pf/internal/check.Provider` or a separate validation helper called by `pkg/pf/tfgen`. Prefer the location that keeps errors build-time and avoids runtime import cycles.
