# Upgrading a Bridged Provider to Plugin Framework

Follow these steps if you have a Pulumi provider that was bridged from a Terraform provider built against
[Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) and you want to upgrade it to a version that has migrated to the Plugin Framework.

1. Ensure you have access to the [`github.com/hashicorp/terraform-plugin-framework/provider.Provider`](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework@v1.13.0/provider#Provider) from
   the upstream provider.  Make sure the module is now depending on
   `"github.com/hashicorp/terraform-plugin-framework"` instead of
   `"github.com/hashicorp/terraform-plugin-sdk/v2"`.  If the provider is shimmed (or needs to be), update the
   source code accordingly. For example, a `shim.go` that looked like this:

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

   Becomes:

    ```go
    package shim

    import (
        "github.com/hashicorp/terraform-plugin-framework/provider"
        p "github.com/hashicorp/terraform-provider-tls/internal/provider"
    )

    func NewProvider() provider.Provider {
        return p.New()
    }
    ```

   Make sure the module builds:

     ```
     cd provider/shim
     go mod tidy
     go build
     ```

2. Find tfgen binary `main` that calls `tfgen.Main` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen`
   and update it to call `tfgen.Main` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen`.

   Note that the extra verson parameter is removed from `tfgen.Main`, so this code:

    ```go
    tfgen.Main("tls", version.Version, tls.Provider())
    ```

   Becomes:

    ```
    tfgen.Main("tls", tls.Provider())
    ```

3. Find the provider binary `main` that calls `tfbridge.Main` from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge` and update it to `Main` from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge`. Note the signature changes: version parameter is removed,
   `Context` is now required, and there is a new `bridge-metadata.json` blob that needs to be embedded:

     ```go
     ...

     func main() {
         meta := tfbridge.ProviderMetadata{PackageSchema: schema}
         tfbridge.Main(context.Background(), "myprovider", myprovider.MyProvider(), meta)
     }
     ```

4. Update code declaring `tfbridge.ProviderInfo` (typically in `provider/resources.go`):

    ```go

    //go:embed cmd/pulumi-resource-myprovider/bridge-metadata.json
    var bridgeMetadata []byte

    func Provider() tfbridge.ProviderInfo {
        info := tfbridge.ProviderInfo{
            // Replace P (abbreviated for Provider):
            P: pf.ShimProvider(shim.NewProvider()).

            // Make sure Version is set, as it is now required.
            Version: ...,

            // This is now required.
            MetadataInfo: tfbridge.NewProviderMetadata(bridgeMetadata),

            // Keep the rest of the code as before.
        }
        return info
    }
    ```

5. From this point the update proceeds as a typical upstream provider update. Build and run the tfgen binary to compute
   the Pulumi Package Schema. It will now also compute a new metadata file `bridge-metadata.json`, build the provider
   binary, re-generate language-specific SDKs and run tests.

    ```
    make tfgen
    make provider
    make build_sdks
    ```
