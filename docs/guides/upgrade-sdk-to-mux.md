# How to Upgrade a provider that has partially migrated to the Plugin Framework

Follow these steps if you have a Pulumi provider that was bridged from a Terraform
provider built against [Terraform Plugin
SDK](https://github.com/hashicorp/terraform-plugin-sdk) and you want to upgrade it to a
version that has migrated some but not all resources/datasources to the [Plugin
Framework](https://github.com/hashicorp/terraform-plugin-framework?tab=readme).

1. Ensure you have access to the
   `github.com/hashicorp/terraform-plugin-framework/provider.Provider` from the upstream
   provider.  If the provider is shimmed (or needs to be), you can follow step (1) from
   ["How to Upgrade a Bridged Provider to Plugin
   Framework"](./upgrade-sdk-to-pf.md).

1. Find the tfgen binary `main` that calls `tfgen.Main` from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen` and update it to call
   `tfgen.MainWithMuxer` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen`.

   Note that the extra version parameter is removed from `tfgen.Main`, so this code:

   ```go
   import "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"

   ...

   tfgen.Main("cloudflare", version.Version, tls.Provider())
   ```

   Becomes:

   ``` go
   import "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"

   ...

   tfgen.MainWithMuxer("cloudflare", cloudflare.Provider())
   ```

1. Find the provider binary `main` that calls
   [`"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge".Main`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge#Main)
   and update it to
   [`"github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge".MainWithMuxer`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge#MainWithMuxer).

   Note the signature changes: version parameter is removed, and `Context` is now required, so this
   code:

     ```go
    import "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"

    ...

     tfbridge.Main("cloudflare", version.Version, cloudflare.Provider(), pulumiSchema)
     ```

     Becomes:

    ```go
    import "github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge"

    ...

    tfbridge.MainWithMuxer(context.Background(), "cloudflare", cloudflare.Provider(), pulumiSchema)
    ```

1. Update code declaring
   [`ProviderInfo`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge#ProviderInfo)
   (typically in `provider/resources.go`), changing the embedded
   `"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge".ProviderInfo.P` to the
   result of calling
   [`"github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge".MuxShimWithPF`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge#MuxShimWithPF).

   This function combines the original SDK based provider with the new PF based provider, so this code:

    ```go
    import (
		"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
		shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"

		"github.com/${PROVIDER_ORG}/terraform-provider-${PROVIDER_NAME}"
	)

    ...

    func Provider() tfbridge.ProviderInfo {
	    p := shimv2.NewProvider(${PROVIDER_NAME}.SDKProvider())

	    prov := tfbridge.ProviderInfo{
			P: p,
	        ...
	    }

		...

		return prov
    }
    ```

    > You should replace
    > `github.com/${PROVIDER_ORG}/terraform-provider-${PROVIDER_NAME}.SDKProvider()` with
    > whatever function is necessary to produce the
    > `*github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema.Provider` used by the
    > upstream provider.

    Becomes:

    ```go
    import (
    	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
       	pfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge"
       	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"

        "github.com/${PROVIDER_ORG}/terraform-provider-${PROVIDER_NAME}"
    )

    ...

    func Provider() tfbridge.ProviderInfo {
        p := pfbridge.MuxShimWithPF(context.Background(),
			shimv2.NewProvider(${PROVIDER_NAME}.SDKProvider()),
			${PROVIDER_NAME}.PFProvider(),
	    )

	    prov := tfbridge.ProviderInfo{
			P: p,
	        ...
	    }

        ...

        return prov
    }
    ```

    > You should replace
    > `github.com/${PROVIDER_ORG}/terraform-provider-${PROVIDER_NAME}.PFProvider()` with
    > whatever function is necessary to produce the
    > `github.com/hashicorp/terraform-plugin-framework/provider.Provider` used by the
    > upstream provider.

1. Ensure that `tfbridge.ProviderInfo.MetadataInfo` is set.

   For details on setting this up, see [here](./metadata.md#setup).

1. From this point the update proceeds as a typical upstream provider update. Build and
   run the tfgen binary to compute the Pulumi Package Schema. It will now also compute a
   new metadata file `bridge-metadata.json`, build the provider binary, re-generate
   language-specific SDKs and run tests.

    ```
    make tfgen
    make provider
    make build_sdks
    ```
