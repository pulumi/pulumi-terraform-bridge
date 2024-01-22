# Automatic Token Mapping

Automatic token mappings provide a programmatic way to map Terraform tokens to Pulumi tokens.

## `tfbridge.ProviderInfo.ComputeTokens`

`tfbridge.ProviderInfo.ComputeTokens` (or `tfbridge.ProviderInfo.MustComputeTokens` to panic on errors) is the entry point for automatic token mapping. `ComputeTokens` takes a strategy, which dictates how each Terraform token maps to Pulumi tokens. There are 4 built-in strategies, which cover most use cases. The interface is open, so you may write a custom strategy.

## Mapping Strategies

[Mapping strategies](https://github.com/pulumi/pulumi-terraform-bridge/blob/5e17c6c7e2d877db7e1d9c0b953a06d3ecabbaea/pkg/tfbridge/tokens.go#L32-L38) dictate the mapping from Terraform to Pulumi tokens. There are 4 Pulumi maintained strategies in the bridge. They live in "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens".

Each built-in strategy decides how to assign Terraform tokens to Pulumi modules, then delegates token construction to a passed in mint function: `func(module, name string) (string, error)`. The standard mint function is `tokens.MakeStandard`.

### `tokens.SingleModule`

`tokens.SingleModule` is a strategy that assigns all tokens to a specific module. 

### `tokens.KnownModules`

`tokens.KnownModules` is a strategy that assigns tokens to a hand-authored set of modules based on prefix in the Terraform token. 

Consider the following example:

```go
strategy := tokens.KnownModules("pkg_", "index", []string{"mod1", "mod2"}, tokens.MakeStandard("pkg"))
```

`strategy` will map `pkg_mod1_resource` to `pkg:mod1/resource:Resource` and `pkg_mod2_datasource` to `pkg:mod2/datasource:getDatasource`. It will map `pkg_mod3_resource` to `pkg:index/mod3Resource:Mod3Resource`, since `mod3` is not in the list of known packages and `"index"` is supplied as a default. If we replace `"index"` with `""`, then `strategy` will error on `pkg:index/mod3Resource:Mod3Resource` instead.

### `tokens.MappedModules`

`tokens.MappedModules` is similar to `tokens.KnownModules`, except it takes a `map[string]string` instead of a `[]string` for it's module set. This allows users to specify a mapping between Terraform prefixes and arbitrary Pulumi modules.

Consider the following example:

```go
m := map[string]string{"mod1": "foo", "mod2": "bar"}
strategy := tokens.KnownModules("pkg_", "index", m, tokens.MakeStandard("pkg"))
```

`strategy` will map `pkg_mod1_resource` to `pkg:foo/resource:Resource` and `pkg_mod2_datasource` to `pkg:bar/datasource:getDatasource`. `tokens.MappedModules` has the same behavior as `tokens.KnownModules` for Terraform tokens that don't have a matching prefix.

### `tokens.InferredModules`

`tokens.InferredModules` attempts to infer which Terraform resources should go into the same module, and then applies that mapping.

> **Warning** `tokens.InferredModules` may decide to move an existing resource when a new resource is added. It should be used with [`tfbridge.ProviderInfo.ApplyAutoAliases`](../pkg/tfbridge/README.md) to prevent breaking changes.
