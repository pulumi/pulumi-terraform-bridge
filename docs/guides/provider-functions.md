# Provider-defined functions

Terraform providers built with the Plugin Framework (protocol 6.5+, Terraform 1.8+) can
define [provider functions](https://developer.hashicorp.com/terraform/plugin/framework/functions/concepts):
pure, offline computations called in Terraform as `provider::<name>::<function>(...)`.
The bridge maps each provider function to a Pulumi invoke.

SDKv2-based providers cannot define functions; muxed providers expose functions from
their Plugin Framework half only.

## Projection

Functions live in a Terraform namespace separate from resources and data sources and are
registered without the provider prefix (for example `parse_arn`, not `aws_parse_arn`).
On the Pulumi side:

- Every token strategy (`SingleModule`, `KnownModules`, `MappedModules`,
  `InferredModules`) maps functions into the top-level (index) module, camelCasing the
  Terraform name with no `get` prefix: `parse_arn` becomes
  `pkg:index/parseArn:parseArn`.
- Ordered Terraform parameters project as positional arguments via the schema's
  `multiArgumentInputs`. Terraform parameter names are documentation-only, so empty or
  duplicate names resolve deterministically (`arg1`, `value`, `value2`, ...).
- A trailing variadic parameter projects to a final, optional list-typed argument.
- The Terraform return type projects via `returnType`, directly (non-object) for scalar
  returns.
- A parameter that sets `AllowNullValue` projects as optional, unless a later parameter
  is required (target languages cannot express a required positional argument after an
  optional one).
- Tuple-typed parameters or returns are not supported and fail schema generation.

## Mapping

Functions have their own section in `ProviderInfo`, keyed by the unprefixed Terraform
name:

```go
Functions: map[string]*tfbridge.FunctionInfo{
    "parse_arn": {Tok: "aws:index/parseArn:parseArn"},
},
```

The `functions` section of the marshaled mapping (`GetMapping`) also records whether the
function's final Terraform parameter is variadic. The Pulumi schema cannot carry this: a
variadic parameter projects as a trailing, optional array argument, indistinguishable
from a genuine trailing list parameter, yet the two take different Terraform call syntax
(spread arguments vs. a list value).

`MustComputeTokens` fills missing entries automatically. Missing and extra mappings are
validated with the same semantics as resources and data sources
(`PULUMI_SKIP_MISSING_MAPPING_ERROR`, `PULUMI_SKIP_EXTRA_MAPPING_ERROR`,
`ProviderInfo.IgnoreMappings`). A function token that collides with a data source invoke
token fails schema generation: both share the `functions` section of the Pulumi schema.

## Runtime

Invoking the generated function routes through the Terraform `CallFunction` RPC.
Functions are pure by the Terraform contract, so the call does not require provider
configuration. Function errors surface as Pulumi errors naming the function; errors
scoped to an argument name the offending Pulumi argument.

## Docs

Upstream documentation is discovered under `website/docs/functions/` (legacy) and
`docs/functions/`, falling back to the `Summary` and `Description` the provider ships in
its schema.
