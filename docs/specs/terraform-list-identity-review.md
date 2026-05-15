# Terraform List Identity Review

## Purpose

This note explains the identity issue behind Terraform list-resource support in
the bridge. It is intentionally less normative than
`terraform-list-runtime-semantics.md`: the goal is to give reviewers enough
context to decide what the first implementation should support and what must be
deferred.

## The ID Terms Are Overloaded

Pulumi and Terraform use similar words for different concepts:

- Pulumi resources require a top-level `id` property in state.
- Pulumi `Read` and import operations pass an import ID string to the provider.
- Terraform Plugin Framework resources may expose structured resource identity.
- Terraform list resources return identity data, and may optionally return a
  full resource object.

These values can coincide, but they are not the same contract. In particular,
the bridge's existing Pulumi `id` extraction path is not proof that the result
can be fed back into Terraform import.

## Current Bridge Behavior

The current PF `Read` import path calls Terraform with a string ID:

- `pkg/pf/tfbridge/provider_read.go` builds
  `tfprotov6.ImportResourceStateRequest{ID: string(id)}`.

Terraform protocol also supports identity-based import:

- `tfprotov6.ImportResourceStateRequest` has an `Identity` field that is
  mutually exclusive with `ID`.

The bridge does not currently use that identity field.

The current list conversion has two paths:

- If Terraform returns a resource object, the bridge decodes it as resource
  state and calls `extractID`.
- If Terraform returns only identity, the bridge synthesizes an opaque
  `identity:<base64>` ID.

Both paths are problematic for importability:

- `extractID` extracts the Pulumi resource ID. It uses `ResourceInfo.ComputeID`
  when configured, otherwise it reads the Pulumi top-level `id` property.
- `ComputeID` exists to satisfy Pulumi's required `id` property. It is often
  compatible with import IDs, but it does not have to be.
- The opaque `identity:` value is not currently decoded by `Read`, so it cannot
  be consumed by Terraform import.

Therefore a list result ID is only correct when it is the same value that the
bridge's `Read` import path expects. A Pulumi state ID or opaque Terraform
identity blob is not enough by itself.

## Terraform List Results

Terraform Plugin Framework list resources receive `ListRequest.IncludeResource`.

When `IncludeResource: false`, Framework converts each result to protocol data
containing:

- display name
- resource identity
- diagnostics

It does not include the resource object.

When `IncludeResource: true`, Framework includes both:

- resource identity
- resource object

The identity is typed by the resource identity schema. It can contain an
attribute named `id`, but there is no general rule that it must. It can also
contain multiple fields such as `account_id`, `region`, names, ARNs, or
provider-specific key fields.

## AWS Provider Evidence

The AWS provider has both SDK-backed and Framework-native list resources.
Current counts from `~/work/pulumi-aws/upstream`:

- `127` total list resources
- `95` `@SDKListResource`
- `32` `@FrameworkListResource`

For the initial bridge version, the important subset is the `32`
Framework-native list resources.

AWS builds Framework resource identity schemas from
`internal/provider/framework/identity/schema.go`, which iterates
`identitySpec.Attributes`. Duplicate attributes such as
`WithIdentityDuplicateAttrs(names.AttrID)` are importer helper behavior, not
fields in the identity data itself.

Among the `32` `FrameworkListResource` entries:

- `10` have an actual `id` attribute in the identity data.
- `22` do not.

Framework list resources whose identity data contains `id`:

- `aws_ec2_network_insights_access_scope`
- `aws_ec2_secondary_network`
- `aws_ec2_secondary_subnet`
- `aws_vpc_security_group_egress_rule`
- `aws_vpc_security_group_ingress_rule`
- `aws_opensearchserverless_collection`
- `aws_opensearchserverless_collection_group`
- `aws_s3files_access_point`
- `aws_s3files_file_system`
- `aws_s3files_mount_target`

Two Framework list resources mention `id` only as a duplicate attribute and
should not be counted as identity data containing `id`:

- `aws_batch_job_queue`
- `aws_s3_directory_bucket`

This means an `identity.id` heuristic would miss most Framework list resources
even in the initial AWS-only evidence set.

## Can The Bridge Detect A Single Identity Parameter?

Partially.

Terraform protocol exposes resource identity schemas separately from normal
resource schemas:

- `GetProviderSchemaResponse.ListResourceSchemas` contains list config schemas.
- `GetResourceIdentitySchemasResponse.IdentitySchemas` contains resource
  identity schemas keyed by Terraform resource name.
- Each `ResourceIdentitySchema` has `IdentityAttributes`.
- Each identity attribute has a name, type, and import requirement flags such
  as `RequiredForImport` and `OptionalForImport`.

That means the bridge can generically inspect the identity schema for a
Terraform resource and classify simple cases:

- If exactly one non-context identity attribute is required for import, the
  bridge can decode the list result identity and extract that value.
- For AWS this identifies cases such as `name`, `bucket`,
  `service_principal`, `file_system_id`, `resource_arn`, or `id`.
- If multiple resource-specific identity attributes are required, the bridge can
  identify that the result is compound and avoid guessing.

This is useful, but it does not prove string import compatibility.

Terraform identity schema tells the bridge which structured identity attributes
are sufficient for identity-based import. It does not say that one of those
attributes is the provider-defined string accepted by
`ImportResourceStateRequest.ID`. The string import format remains
provider-defined.

The distinction is:

- A single required identity attribute proves the provider can identify the
  resource from that structured identity field.
- It does not generically prove that passing that field as the string import ID
  is valid.

For AWS, the shortcut often works because its generic single-parameter and ARN
import helpers accept the same value as `request.ID`. For example, a
single-parameter identity on `bucket` imports from the bucket name, and an ARN
identity imports from the ARN string. That is AWS implementation behavior, not
a Terraform protocol guarantee.

Therefore a generic bridge implementation can safely use identity schemas for
classification and for identity-aware import. It should treat "extract the only
required identity attribute and use it as the Pulumi list result ID" as a
temporary compatibility shortcut, not the end-state model.

If such a shortcut is used before identity-aware `Read` exists, it should be
guarded narrowly:

1. exactly one required identity attribute;
2. value is a primitive scalar that can be stringified predictably;
3. no provider-specific compound import handler is needed;
4. failures are clear and fail closed; and
5. tests cover both a working single-parameter case and a compound-identity
   case that must not guess.

## SDKv2 Resource State Versus SDKv2 List Identity

SDKv2 resources add another easy-to-miss distinction.

Terraform SDKv2 resource state has a special resource ID channel:

- `ResourceData.Id()` reads from `InstanceState.ID` and falls back to the
  top-level `id` state attribute.
- `ResourceData.SetId(...)` sets `InstanceState.ID` and writes the top-level
  `id` state attribute.
- `Resource.CoreConfigSchema()` adds an implicit top-level `id` attribute when
  the provider schema does not define one.

So for SDKv2 resource *objects*, the top-level `id` is effectively always
present as part of the SDKv2 state model.

That does not mean SDKv2-backed *list identity* always contains `id`.

In terraform-provider-aws, SDK-backed list resources are wrapped by
`ListResourceWithSDKv2Resource`. The wrapper builds a Terraform resource
identity schema from the AWS `identitySpec.Attributes`, not from the SDKv2
implicit state `id`. The AWS SDK identity interceptor only reads
`ResourceData.Id()` when the identity attribute maps to resource attribute
`id`; otherwise it reads whatever resource attribute the identity spec names.

Current AWS evidence:

- `95` `@SDKListResource` entries.
- `22` have an actual `id` attribute in identity data.
- `73` do not.

Examples of SDK-backed list identities without `id`:

- `aws_appflow_flow` uses `name`.
- `aws_cloudwatch_metric_alarm` uses `alarm_name`.
- `aws_dynamodb_table` uses `name`.
- `aws_ecr_repository` uses `name`.
- `aws_api_gateway_integration` uses compound identity fields.
- `aws_route` uses compound identity fields.
- `aws_volume_attachment` uses compound identity fields.

The practical implication is:

- With `IncludeResource: true`, SDK-backed list results can provide a resource
  object with top-level SDKv2 state `id`.
- With `IncludeResource: false`, SDK-backed list results still have the same
  identity-schema problem as Framework-native list results.

## Why Resource Objects Do Not Fully Solve It

Requesting `IncludeResource: true` is still useful because it lets the bridge
decode a resource-shaped result. That avoids some identity-only limitations and
lets the bridge reuse existing resource decoding and state transforms.

However, resource objects still only expose Pulumi state. The current bridge
then calls `extractID`, which extracts the Pulumi resource ID. That value might
be the import ID, but it is not guaranteed to be.

This distinction matters most for Plugin Framework resources because Terraform
Framework does not require a resource field named `id`. When the upstream
resource has no usable `id`, the bridge may synthesize a Pulumi ID through
`ComputeID` or `MissingIDComputeID`. That keeps Pulumi state valid, but it does
not automatically provide the string Terraform import expects.

## What Would Be Safe For The First Version

The first version should fail closed unless it can produce an ID that the
current bridge `Read` path can actually import.

Recommended first-version rules:

1. Request `IncludeResource: true` so Terraform returns resource objects when
   it can.
2. Do not accept identity-only results by synthesizing `identity:<base64>` IDs.
3. For identity-only results, support only the narrow single-required-scalar
   shortcut:
   - fetch the Terraform resource identity schema;
   - require exactly one required identity attribute;
   - require the decoded identity value to be a primitive scalar;
   - stringify that value as `ListResponse.Result.Id`; and
   - fail closed for compound identity, missing schemas, decode failures, or
     non-scalar values.
4. Treat this as a compatibility shortcut, not proof that Terraform identity
   attributes are generally string import IDs.
5. Treat `extractID` as Pulumi-ID extraction, not import-ID extraction.

For AWS Framework list resources, this covers the `10` resources whose identity
attribute is named `id` plus `16` more resources with a single non-`id` identity
parameter or ARN. The remaining `6` Framework list resources have compound
identity and should fail closed until the bridge has identity-aware import/read
support or explicit provider metadata for formatting import IDs.

## Deferred Design: Identity-Aware Read

The cleaner long-term design is to preserve Terraform identity data from
identity-only list results and teach `Read` to consume it.

That would require:

- encoding the Terraform identity data into the Pulumi list result ID or another
  stable continuation/import carrier;
- decoding that value in the bridge `Read` path;
- calling `ImportResourceStateRequest{Identity: ...}` instead of `ID: ...`;
- preserving enough schema/version information to make the identity dynamic
  value meaningful; and
- deciding how this interacts with existing Pulumi import strings and provider
  `ComputeID` behavior.

Until that design exists, identity-only list results outside the
single-required-scalar shortcut should be reported as unsupported instead of
appearing to work with an opaque ID that later fails on `Read`.

## Open Review Questions

1. Should Phase 1 require explicit metadata for importable list IDs, or is it
   acceptable to use the current Pulumi `id` extraction for Framework resources
   whose providers conventionally use the same value for import?
2. If we keep using Pulumi `id` extraction temporarily, what test fixture proves
   the failure mode where Pulumi ID and import ID differ?
3. Should identity-aware import be part of the same feature stack, or should it
   be a separate follow-up after resource-object list support lands?
4. Do we need a provider-facing escape hatch such as `ListID`, or should this
   reuse existing `ComputeID` with clearer documentation about importability?
