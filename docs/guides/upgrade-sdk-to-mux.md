# Upgrading a Partially Migrated Provider with the PF Muxer

Follow these steps when an upstream Terraform provider has migrated some, but not
all, resources or data sources from Terraform Plugin SDKv2 to
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).
The resulting Pulumi provider dispatches each Terraform token to either the SDKv2
or Framework implementation.

The examples below use SDKv2. Providers still using Terraform Plugin SDKv1 can
follow the same structure with
`github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1` in place of the
SDKv2 shim.

1. Obtain both upstream provider implementations:

   - an SDK provider, normally a
     `*github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema.Provider`; and
   - a `github.com/hashicorp/terraform-plugin-framework/provider.Provider`.

   If either constructor is in a Go `internal` package, expose it through a small
   shim module as described in step 1 of the
   [PF-only migration guide](./upgrade-sdk-to-pf.md). Real providers commonly pass
   a version or `context.Context` to these constructors, or construct the Framework
   provider from the SDK provider so both share configuration. Preserve that
   upstream initialization rather than assuming the constructors are independent.

2. Set up provider metadata.

   Muxed providers require generated bridge metadata for their dispatch table.
   Bootstrap the file before adding a `go:embed` directive:

   ```sh
   echo '{}' > provider/cmd/pulumi-resource-myprovider/bridge-metadata.json
   ```

   Adjust the path if running from the `provider` directory. See
   [Provider Metadata](./metadata.md) for the complete setup.

3. Update the code that declares `tfbridge.ProviderInfo`, typically
   `provider/resources.go`, to combine the two implementations:

   ```go
   package myprovider

   import (
       "context"
       _ "embed"

       pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
       "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
       shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"

       "github.com/pulumi/pulumi-myprovider/provider/pkg/version"
       "github.com/pulumi/pulumi-myprovider/provider/upstream"
   )

   //go:embed cmd/pulumi-resource-myprovider/bridge-metadata.json
   var bridgeMetadata []byte

   func Provider() tfbridge.ProviderInfo {
       ctx := context.Background()
       sdkProvider := upstream.SDKProvider()
       frameworkProvider := upstream.PFProvider()

       p := pftfbridge.MuxShimWithPF(
           ctx,
           shimv2.NewProvider(sdkProvider),
           frameworkProvider,
       )

       return tfbridge.ProviderInfo{
           P: p,

           // The mux entrypoints read the version from ProviderInfo rather than
           // receiving it as a separate argument.
           Version: version.Version,

           MetadataInfo: tfbridge.NewProviderMetadata(bridgeMetadata),

           // Keep the rest of the provider mapping as before.
       }
   }
   ```

   Replace `upstream.SDKProvider()` and `upstream.PFProvider()` with the constructors
   exposed by the upstream provider or its shim. It is also reasonable for
   `Provider` to accept a `context.Context` and pass it to both constructors.

   Choose the mux helper according to the upstream ownership model:

   - `MuxShimWithPF` permits overlapping Terraform tokens. When both providers expose
     a token, the SDK implementation wins.
   - `MuxShimWithDisjointgPF` requires disjoint token sets and panics on overlap. Use
     it when the upstream migration is expected to have exactly one owner for every
     token; it catches ownership mistakes early. `DisjointgPF` is the current
     exported spelling.

4. Update the tfgen binary to use `pkg/pf/tfgen.MainWithMuxer`. Remove the separate
   version argument because it now comes from `ProviderInfo.Version`:

   ```go
   import pftfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"

   func main() {
       pftfgen.MainWithMuxer("myprovider", myprovider.Provider())
   }
   ```

   This replaces the SDK form:

   ```go
   tfgen.Main("myprovider", version.Version, myprovider.Provider())
   ```

5. Update the provider binary to use
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge.MainWithMuxer`.
   It requires a `context.Context`, takes the version from `ProviderInfo`, and
   receives the generated Pulumi schema directly:

   ```go
   import (
       "context"
       _ "embed"

       pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
   )

   //go:embed schema-embed.json
   var pulumiSchema []byte

   func main() {
       pftfbridge.MainWithMuxer(
           context.Background(),
           "myprovider",
           myprovider.Provider(),
           pulumiSchema,
       )
   }
   ```

   `MainWithMuxer` and the corresponding tfgen entrypoint are currently
   experimental bridge APIs, although they are used by production providers.

6. Continue with the normal upstream-provider update. Tfgen computes the Pulumi
   package schema and mux dispatch metadata. Build the provider and SDKs and run
   tests:

   ```sh
   make tfgen
   make provider
   make build_sdks
   make test
   ```

   Review the generated `bridge-metadata.json` and schema changes before committing
   them. In particular, confirm that resources and data sources moved upstream are
   dispatched to the Framework side and unchanged tokens remain on the SDK side.
