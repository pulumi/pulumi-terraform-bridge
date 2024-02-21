# Provider Metadata

Bridged providers may need to persist information between different invocations of `make
tfgen`, or between tfgen time and normal runtime. The bridge provides a mechanism to do
that: metadata.

Provider metadata is required for PF based providers, muxed providers and [AutoAliasing](./auto-aliasing.md).

## Setup

Adding provider metadata has three steps.

1. Creating the metadata file.
2. Embedding the metadata file.
3. Giving `ProviderInfo` a reference to the metadata file.


### Creating the metadata file

The bridge expects the metadata file to live next to `schema.json` at
`cmd/pulumi-resource-${PROVIDER_NAME}/bridge-metadata.json` (where `${PROVIDER_NAME}` is
replaced with the name of your provider). To bootstrap the file, the metadata file should
be an empty JSON file.

To create the metadata file, run:

```sh
echo {} > cmd/pulumi-resource-${PROVIDER_NAME}/bridge-metadata.json
```

### Embedding the metadata file

The bridge uses [go:embed](https://pkg.go.dev/embed) to access `bridge-metadata.json` at
runtime. To embed a `bridge-metadata.json`, add `_ "embed"` to the imports section of `resources.go`:


```go

import (
	_ "embed"

    ...
)
```

> The `_` in `_ "embed"` tells Go that you don't need to use any items from `"embed"`,
> just to import the module.

Finally, we need to link the file to a variable so the Go compiler includes it during builds:

```go
//go:embed cmd/pulumi-resource-${PROVIDER_NAME}/bridge-metadata.json
var metadata []byte
```

### Giving `ProviderInfo` a reference to the metadata file

    Now that `bridge-metadata.json` is embedded as the `metadata` variable, we need to give the returned `tfbridge.ProviderInfo` access. We do that by setting the `MetadataInfo` field on `tfbridge.ProviderInfo`:

```
import "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"

func Provider() tfbridge.ProviderInfo {
	info := tfbridge.ProviderInfo{
    	MetadataInfo: tfbridge.NewProviderMetadata(metadata),
        ...
    }

    ...

	return info
}
```
