# GH-3428 Lazy PF Schema Runtime Semantics

## Summary

Issue [#3428](https://github.com/pulumi/pulumi-terraform-bridge/issues/3428) tracks a memory cost in muxed providers that include Terraform Plugin Framework resources, data sources, or list resources. Before this change, runtime startup materialized the full PF provider schema in the long-lived Pulumi provider process, even when a Pulumi program used only SDKv2 resources or only a small subset of PF resources.

Local experiments on a large muxed provider showed provider-process RSS reductions of roughly 120-180 MiB when runtime startup avoided eager full-provider PF schema materialization. The target model is:

1. Build-time schema generation still materializes and validates the full provider schema.
2. Runtime startup gathers cheap PF metadata needed for dispatch and type-name resolution.
3. Runtime startup does not call full PF `GetProviderSchema`.
4. Runtime operations lazily materialize only the PF resource, data source, or list resource schemas they use.
5. SDKv2-only programs in muxed providers do not pay the full PF schema cost.
6. Generated Pulumi schema and bridge metadata remain unchanged.

This is a runtime memory change, not a schema-generation behavior change.

## Implemented Flow

### Build-Time Schema Generation

PF schema generation enters through `pkg/pf/tfgen`.

- `pkg/pf/tfgen/gen.go` calls `check.Provider` before delegating to the standard `pkg/tfgen` generator.
- `pkg/tfgen` walks `info.P.Schema()`, `info.P.ResourcesMap()`, `info.P.DataSourcesMap()`, and list-resource metadata to build the Pulumi package schema.
- For PF providers, `info.P` is a schema-only shim built by `pkg/pf/internal/schemashim`.

With lazy runtime schema loading, tfgen still materializes resource, data source, and list resource schemas because schema generation needs complete Pulumi package schema output.

`check.Provider` now runs Framework implementation validation at build time for the provider schema and every generated PF resource, data source, and list resource schema. Runtime no longer relies on full provider-wide `GetProviderSchema` as the place where Framework `ValidateImplementation` failures are discovered for static generated providers.

### Runtime Startup

PF runtime providers are constructed in `pkg/pf/tfbridge/provider.go`.

1. `newProviderWithContext` asserts `info.P` implements `pf.ShimProvider`.
2. It calls `pfServer.Server(ctx)` to create a Terraform protocol v6 server.
3. It calls `pfServer.Resources(ctx)` and `pfServer.DataSources(ctx)` to get runtypes collections.
4. It calls `pfServer.Config(ctx)` to get provider config type information.
5. It creates config/resource/data-source/list encoders later from the selected schema types.

`SchemaOnlyProvider.Server` creates a Framework protocol server with `providerserver.NewProtocol6(p.tf)` and does not call full `GetProviderSchema`. A regression test covers the old provider-name initialization problem where a resource name such as `random_integer` could become `_integer`.

Avoiding full `GetProviderSchema` is the memory-reduction point. In Terraform Plugin Framework, `GetProviderSchema` walks provider schema, provider meta schema, every resource schema, every data source schema, functions, ephemeral resources, list resources, actions, and state stores. It also validates schema implementation while doing so; that validation now belongs in tfgen/build time for static generated providers.

### PF Metadata and Per-Entity Schema Loading

Current Terraform Plugin Framework versions expose a cheaper metadata path:

- `GetMetadata` enumerates resource and data source type names and server capabilities.
- Per-resource server operations can later call `ResourceSchema(typeName)`, which loads and caches only the selected resource schema.
- Per-data-source operations similarly load only the selected data source schema.
- List-resource schema loading can be driven from the selected schema-only list-resource map entry without loading the full provider schema.

The bridge already has enough local metadata gathering to build resource, data source, and list resource maps without calling schema methods:

- `GatherResources` calls provider `Metadata`, provider `Resources`, and each resource `Metadata`.
- `GatherDatasources` calls provider `Metadata`, provider `DataSources`, and each data source `Metadata`.
- `GatherListResources` calls provider `Metadata`, provider `ListResources`, and each list resource `Metadata`.

Those gatherers store a lazy schema adapter for each map entry. Methods such as `Type`, `Attrs`, `Blocks`, `Shim`, `ResourceSchemaVersion`, `DeprecationMessage`, and `ResourceProtoSchema` force the real schema only when used.

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

### Runtime List Operations

PF List builds its result resource handle from the selected CRUD resource schema and its query encoder from the selected list resource schema:

- Resolve Pulumi token to Terraform resource type name.
- Fetch the selected CRUD schema for list result decoding.
- Fetch the selected list resource schema from the schema-only list resource map.
- Encode the Pulumi list query using the native lazy Framework list schema.
- Call PF `ValidateListResourceConfig` and `ListResource`.

Lazy mode should load only the selected CRUD resource schema and selected list resource schema. It should not call full provider `GetProviderSchema`, and it should not load unrelated PF resource, data source, or list resource schemas.

### Muxed Providers

Muxed providers combine SDKv2 and PF providers.

- `pkg/pf/internal/muxer` extends a base shim with a PF schema-only provider.
- Dispatch metadata chooses the sub-provider for each Pulumi resource or function.
- SDKv2 should dominate conflicts.

The critical GH-3428 case is a muxed provider where the user program touches only SDKv2 resources. Runtime must still know PF resource, data source, and list resource type names for dispatch metadata and aliases, but it should not materialize all PF schemas.

## Desired Semantics

1. `tfgen` and build-time helper binaries fully materialize the provider schema and generate the same Pulumi package schema as before.
2. Build-time should run bridge PF validation and Framework schema implementation validation so invalid provider schemas fail before release.
3. Runtime startup should not call PF full `GetProviderSchema` for static generated providers.
4. Runtime startup may call metadata-only APIs or local provider/resource/data source `Metadata` methods to establish type-name maps.
5. Runtime provider config schema may remain eager because provider configuration is needed for `CheckConfig`, `Configure`, and config encoding.
6. PF resource schemas should be loaded only when a PF resource operation actually needs that resource.
7. PF data source schemas should be loaded only when a PF invoke actually needs that data source.
8. PF list resource schemas should be loaded only when a PF List operation actually needs that list resource.
9. SDKv2-only operations in muxed providers should not call PF resource, data source, or list resource `Schema` methods.
10. Mux dispatch, alias resolution, conflict detection, and resource/data-source/list-resource map enumeration should not force PF schemas.
11. Existing PF lifecycle behavior must remain unchanged for used PF resources, data sources, and list resources.
12. Generated schema, bridge metadata, and dispatch metadata should remain unchanged.
13. Dynamic bridge schema compatibility must not drift unless explicitly accepted.

## Validation Boundary

### Build-Time Validation

Build time validates provider schema implementation because Pulumi static providers already separate schema generation from runtime execution.

Required build-time checks:

- Existing bridge PF checks in `pkg/pf/internal/check`.
- Framework provider schema implementation validation.
- Framework resource schema implementation validation for every generated PF resource.
- Framework data source schema implementation validation for every generated PF data source.
- Framework list resource schema implementation validation for every generated resource that has a PF list resource.

If Framework validation fails during a provider upgrade, the actionable outcomes are pinning, patching, reporting upstream, or waiting for a fixed upstream release. Runtime cannot repair an invalid upstream provider schema, so runtime should not pay full provider-wide validation cost on every provider process startup.

This is a static generated-provider guarantee. Build-time validation should catch invalid schemas for the released provider binary, but runtime lazy loading can still fail if schema construction depends on runtime-only environment, parameterization, dynamic-provider discovery, or code paths that differ from tfgen.

### Runtime Startup Validation

Runtime startup should validate only what it needs to start safely:

- provider metadata and type-name availability;
- provider config schema/type information;
- bridge metadata presence;
- mux dispatch table presence for muxed providers.

Runtime startup should not validate every PF resource, data source, or list resource schema.

### Runtime User-Input Validation

Runtime must still run user-input validations because these depend on the Pulumi program:

- `CheckConfig` and PF `ValidateProviderConfig`;
- `Check` and PF `ValidateResourceConfig`;
- `Invoke` and PF `ValidateDataResourceConfig`;
- `List` and PF `ValidateListResourceConfig`;
- diagnostics processing and CheckFailure conversion.

These are not replaceable by tfgen because they validate actual config, inputs, unknowns, secrets, defaults, and provider-specific diagnostics.

## Design

### Build-Time Framework Validation

The PF build-time path runs explicit Framework implementation validation from `pkg/pf/internal/check.Provider`.

The helper:

1. Detects PF-backed providers and PF-backed entries in muxed providers.
2. Validates provider schema.
3. Iterates PF resources that are included in generation and validates each resource schema.
4. Iterates PF data sources that are included in generation and validates each data source schema.
5. Iterates PF list resources that are included in generation and validates each list resource schema.
6. Converts Framework diagnostics into bridge errors with provider, resource, data source, or list resource names.

Framework `ValidateImplementation` methods operate on the concrete Framework schema values (`provider/schema.Schema`, `resource/schema.Schema`, `datasource/schema.Schema`, and `list/schema.Schema`). The schema-only shim exposes the original Framework provider through an internal hook so build-time validation can call the concrete Framework schema methods without broadening the public shim interfaces.

The canary tests prove `pkg/pf/tfgen.GenerateSchema` fails for PF schemas that Framework `ValidateImplementation` rejects, such as an optional string attribute with a default but without `Computed`, tokenless mapped PF entries, invalid provider schema, invalid data source schema, and invalid list resource schema.

### Lazy Bridge Schema Adapters

The lazy adapter shape is:

- gather metadata eagerly;
- store `tfName` eagerly;
- store a mutex-guarded cached loader for the real PF schema;
- materialize on calls to `Type`, `Attrs`, `Blocks`, `Shim`, `ResourceSchemaVersion`, `DeprecationMessage`, and `ResourceProtoSchema`.

The current `runtypes.Schema` interface does not return errors from those methods, so schema-load failures either need to panic or require a broader interface change. For this issue, panic-on-load-error is acceptable only if build-time validation is already responsible for catching invalid provider schemas before release. Runtime panics from schema loading should still be wrapped by the provider server panic recovery path.

The cached loader records successful loads and non-cancellation failures. If the first load fails because a caller-supplied context was already canceled, that failure is not cached; a later uncanceled runtime operation can retry the schema load. Construction contexts are detached from cancellation so a canceled startup or gather context does not poison future lazy loads.

### Runtime Server Initialization

`SchemaOnlyProvider.Server` creates the Framework protocol server without a full `GetProviderSchema` initialization call. Direct local metadata gathering is sufficient for static generated providers, and static runtime startup does not use full `GetProviderSchema`.

The implementation includes a regression test for the old provider-name problem:

- define a PF provider whose resource `Metadata` uses `req.ProviderTypeName + "_thing"`;
- create the runtime server without full `GetProviderSchema`;
- call a resource RPC such as `ValidateResourceConfig` or `PlanResourceChange`;
- assert the server recognizes `test_thing`, not `_thing`.

### Muxed Provider Behavior

For muxed providers:

- PF metadata collection may happen during augmentation.
- Dispatch resolution must not call PF resource, data source, or list resource `Schema`.
- SDKv2-only operations must route to the SDKv2 provider without loading PF schemas.
- PF resource, data source, and list operations may load the selected PF schema and only that schema.

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

Dropping runtime `GetProviderSchema` removes provider-wide Framework schema implementation validation from runtime startup. Equivalent validation is now in tfgen/build time for static providers.

### Provider Name Initialization

The previous implementation used `GetProviderSchema` to avoid "Resource Type Not Found" caused by missing provider type names. Current Framework code has metadata-only paths, and the branch includes a regression test that validates resource type-name resolution without full schema initialization.

### Lazy Error Timing

Errors from a bad resource, data source, or list resource schema move from provider shim construction or startup to first schema use. Build-time validation should prevent this in released static providers, and tests pin panic/error behavior for lazy loads.

### SDKv2/PF Parity

SDKv2 schema behavior should not be changed by PF lazy schema work. Any SDKv2 laziness changes should be separate unless directly required.

### Dynamic Bridge

Dynamic providers may depend on full runtime provider schema discovery. The lazy static-provider path should not silently change dynamic schema generation or dynamic provider runtime behavior.

### Mux Dispatch and Aliases

Alias insertion and dispatch resolution use resource/data source maps. Tests must prove those operations remain metadata-only and do not force PF schema loading.

### State Upgrade and Raw State

PF state parsing, `UpgradeResourceState`, `ResourceSchemaVersion`, `RawStateDelta`, and private state all use the selected resource schema. Lazy loading is fine for a used resource, but tests need to cover these paths.

### Diagnostics Timing

Framework schema implementation diagnostics previously appeared during runtime startup because of `GetProviderSchema`. They now appear during tfgen/build time for static generated providers. User-input diagnostics must continue to appear during runtime operations.

## Rejected Shortcuts

### Keep Full Runtime `GetProviderSchema`

This preserves current validation and initialization behavior, but it keeps the memory problem. It is not a solution to GH-3428.

### Only Add Lazy Wrappers

Lazy bridge wrappers alone are insufficient if runtime startup still calls full PF `GetProviderSchema`, because that call materializes every PF resource, data source, and list resource schema.

### Drop Validation Without Moving It

Skipping runtime `GetProviderSchema` without adding build-time Framework validation weakens provider release checks. Runtime cannot fix invalid upstream schema implementation, but the build should catch it.

### Use Generated Pulumi Schema For All Runtime Encoding

Generated Pulumi schema is not a complete replacement for PF runtime schema because PF protocol calls need Terraform types, schema versions, and conversion details for the selected resource, data source, or list resource.

### Treat Dynamic Bridge Like Static Generated Providers

Dynamic providers have different schema discovery constraints. Do not apply static-provider assumptions to dynamic bridge without a dedicated compatibility check.

## Test Plan

### Build-Time Tests

1. **Framework validation canary**
   - Add PF provider, resource, data source, and list resource schemas that Framework `ValidateImplementation` rejects.
   - Run `pkg/pf/tfgen.GenerateSchema`.
   - Assert generation fails with a diagnostic identifying the invalid schema.

2. **Tokenless generated mapping validation**
   - Add non-nil `ProviderInfo.Resources` and `ProviderInfo.DataSources` entries whose tokens are empty and would use default naming.
   - Assert invalid PF resource, data source, and list resource schemas still fail build-time validation.

3. **Unmapped skip behavior**
   - Add an invalid PF resource that is not generated.
   - Assert build-time validation skips it.

4. **Valid schema still generates**
   - Existing PF tfgen tests continue to pass.

### Lazy Schema Unit Tests

1. Construct a PF schema-only provider with multiple counted resources, data sources, and list resources.
2. Assert `ShimSchemaOnlyProvider` calls provider/resource/data-source/list-resource metadata but not schema methods.
3. Assert `ResourcesMap().Len`, `DataSourcesMap().Len`, `ListResourcesMap().Len`, `Range`, and `GetOk` do not load schemas.
4. Assert accessing one resource schema loads exactly that resource once.
5. Assert accessing one data source schema loads exactly that data source once.
6. Assert accessing one list resource schema loads exactly that list resource once.
7. Assert concurrent access to the same schema loads once.
8. Assert a canceled construction context does not poison later lazy loads.
9. Assert a caller-context cancellation failure is not cached and can be retried.

### Runtime Startup Tests

1. Start a PF provider server without full `GetProviderSchema`.
2. Assert provider config type and config validation still work.
3. Assert resource type-name resolution works for a resource whose metadata depends on provider type name.
4. Assert direct server resource validation succeeds without full schema initialization.

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
2. Run an SDKv2-only operation and assert no PF resource, data source, or list resource schema loads.
3. Run one PF resource operation and assert only that resource schema loads.
4. Run one PF data source invoke and assert only that data source schema loads.
5. Run one PF List operation with non-empty scalar and list-valued query inputs and assert only the selected CRUD resource and list resource schemas load.
6. Assert alias and dispatch resolution do not force PF schema loading.

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

## Decisions Made

1. Lazy schema load failures continue to panic because `runtypes.Schema` does not expose error-returning accessors. Build-time Framework validation is the release-time guard for static generated providers.
2. Runtime server initialization relies on direct local metadata and Framework per-RPC metadata paths. It does not call `GetMetadata` or full `GetProviderSchema` for static startup.
3. Framework validation lives in `pkg/pf/internal/check.Provider`, which keeps the validation boundary in the build-time check path before schema generation.
