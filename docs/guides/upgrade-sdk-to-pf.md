# Upgrading a Bridged Provider to Plugin Framework

Follow these steps when an upstream Terraform provider has migrated all of the
resources and data sources used by the Pulumi provider from Terraform Plugin SDKv2
to [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).
If the upstream provider has migrated only part of its surface, follow the
[muxed-provider guide](./upgrade-sdk-to-mux.md) instead.

1. Obtain the upstream
   [`provider.Provider`](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/provider#Provider).

   Add `github.com/hashicorp/terraform-plugin-framework` to the module that exposes
   the upstream provider. Once that module no longer imports SDKv2 directly, `go mod
   tidy` can remove its direct `github.com/hashicorp/terraform-plugin-sdk/v2`
   dependency.

   When the upstream constructor is in a Go `internal` package, a small shim module
   can expose it. For example, an SDKv2 shim such as:

   ```go
   package shim

   import (
       "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
       "github.com/hashicorp/terraform-provider-tls/internal/provider"
   )

   func NewProvider() *schema.Provider {
       p, _ := provider.New()
       return p
   }
   ```

   becomes:

   ```go
   package shim

   import (
       "github.com/hashicorp/terraform-plugin-framework/provider"
       tlsprovider "github.com/hashicorp/terraform-provider-tls/internal/provider"
   )

   func NewProvider() provider.Provider {
       return tlsprovider.New()
   }
   ```

   Providers that already export a Framework constructor do not need a separate
   shim module. Provider constructors also commonly require a version or a
   `context.Context`; preserve those arguments when exposing the constructor.

   From the repository root, make sure the shim module builds:

   ```sh
   (cd provider/shim && go mod tidy && go build ./...)
   ```

2. Set up provider metadata.

   Plugin Framework providers require generated bridge metadata. Bootstrap the file
   before adding a `go:embed` directive, otherwise the provider package cannot
   compile and tfgen cannot run:

   ```sh
   echo '{}' > provider/cmd/pulumi-resource-myprovider/bridge-metadata.json
   ```

   Adjust the path if running from the `provider` directory. See
   [Provider Metadata](./metadata.md) for the complete setup.

3. Update the code that declares `tfbridge.ProviderInfo`, typically
   `provider/resources.go`:

   ```go
   package myprovider

   import (
       _ "embed"

       pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
       "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"

       "github.com/pulumi/pulumi-myprovider/provider/pkg/version"
       "github.com/pulumi/pulumi-myprovider/provider/shim"
   )

   //go:embed cmd/pulumi-resource-myprovider/bridge-metadata.json
   var bridgeMetadata []byte

   func Provider() tfbridge.ProviderInfo {
       return tfbridge.ProviderInfo{
           // Replace the SDKv2 shim with the Plugin Framework shim.
           P: pftfbridge.ShimProvider(shim.NewProvider()),

           // The PF entrypoints read the version from ProviderInfo rather than
           // receiving it as a separate argument.
           Version: version.Version,

           MetadataInfo: tfbridge.NewProviderMetadata(bridgeMetadata),

           // Keep the rest of the provider mapping as before.
       }
   }
   ```

4. Update the tfgen binary to use `pkg/pf/tfgen`. The PF entrypoint reads the
   version from `ProviderInfo`, so remove the separate version argument:

   ```go
   import pftfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"

   func main() {
       pftfgen.Main("myprovider", myprovider.Provider())
   }
   ```

   This replaces the SDKv2 form:

   ```go
   tfgen.Main("myprovider", version.Version, myprovider.Provider())
   ```

5. Update the provider binary to use `pkg/pf/tfbridge.Main`. It requires a
   `context.Context` and `ProviderMetadata` containing the generated Pulumi package
   schema:

   ```go
   import (
       "context"
       _ "embed"

       pftfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
   )

   //go:embed schema-embed.json
   var pulumiSchema []byte

   func main() {
       meta := pftfbridge.ProviderMetadata{PackageSchema: pulumiSchema}
       pftfbridge.Main(context.Background(), "myprovider", myprovider.Provider(), meta)
   }
   ```

   `bridge-metadata.json` is supplied through `ProviderInfo.MetadataInfo` from step
   3. Do not add it to `ProviderMetadata.BridgeMetadata`; that field is deprecated.

6. Continue with the normal upstream-provider update. Generate the schema and bridge
   metadata, build the provider and SDKs, and run tests:

   ```sh
   make tfgen
   make provider
   make build_sdks
   make test
   ```

   Review the generated `bridge-metadata.json` and schema changes before committing
   them.
