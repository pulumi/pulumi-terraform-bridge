# Pulumi Bridge for Terraform Plugin Framework

This bridge enables creating [Pulumi Resource
providers](https://www.pulumi.com/docs/intro/concepts/resources/providers/) from [Terraform
Providers](https://github.com/terraform-providers) built using the [Terraform Plugin
Framework](https://developer.hashicorp.com/terraform/plugin/framework).

This bridge is in active development and has an incomplete feature set. Progress is tracked in
[#744](https://github.com/pulumi/pulumi-terraform-bridge/issues/744).

If you need to adapt Terraform providers to Pulumi today see [Pulumi Terraform
Bridge](https://github.com/pulumi/pulumi-terraform-bridge) which only works with providers built with the [Terraform
Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) but is complete.

## How to Bridge a Provider

Follow these steps to bridge a Terraform Provider to Pulumi.

1. You will need a Provider value from the `github.com/hashicorp/terraform-plugin-framework/provider` package. You can
   build it yourself as part of developing a Terraform Provider, or find it in published Terraform sources.

   For example, `terraform-provider-random`
   [exposes](https://github.com/hashicorp/terraform-provider-random/blob/main/internal/provider/provider.go#L13) a `func
   New() provider.Provider` call. Since this definition lives in an `internal` package it cannot easily be referenced in
   an external Go project, but it is still possible to reference it using Go linker tricks. See
   `tests/internal/randomshim/shim.go` for a full example.

2. Populate a `ProviderInfo` struct, mapping Terraform resource names to Pulumi tokens. Replace `myprovider` with your
   provider name.

    ```go
    package myprovider

    import (
        _ "embed"
        "github.com/hashicorp/terraform-plugin-framework/provider"
        pf "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
        "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
    )

    //go:embed cmd/pulumi-resource-myprovider/bridge-metadata.json
    var bridgeMetadata []byte

    func MyProvider() pf.ProviderInfo {
        info := tfbridge.ProviderInfo{
            Name:    "myprovider",
            Version: "1.2.3",
            Resources: map[string]*tfbridge.ResourceInfo{
                "myresource": {Tok: "myprovider::MyResource"},
            },
            MetadataInfo: tfbridge.NewProviderMetadata(bridgeMetadata),
        }
        return pf.ProviderInfo{
            ProviderInfo: info,
            NewProvider: func() provider.Provider {
                return nil // TODO fill in Terraform Provider from Step 1
            },
        }
    }
    ```

3. Build a `pulumi-tfgen-myprovider` binary.

    ```go
    package main

    import (
        "github.com/pulumi/pulumi-terraform-bridge/pf/tfgen"
        // import myprovider
    )

    func main() {
        tfgen.Main("myprovider", myprovider.MyProvider())
    }
    ```

4. Generate a [Pulumi Package Schema](https://www.pulumi.com/docs/guides/pulumi-packages/schema/) and bridge metadata.

    ```bash
    mkdir -p ./schema
    pulumi-tfgen-myprovider schema --out ./schema
    jq . ./schema/schema.json
    jq . ./schema/bridge-metadata.json
    ```

5. Build the Pulumi provider binary `pulumi-resource-myprovider`, embedding the generated `schema.json` and
   `bridge-metadata.json` from Step 4.

    ```go
    package main

    import (
        "context"
        _ "embed"

        "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
        // import myprovider
    )

    //go:embed schema.json
    var schema []byte

    func main() {
        meta := tfbridge.ProviderMetadata{PackageSchema: schema}
        tfbridge.Main(context.Background(), "myprovider", myprovider.MyProvider(), meta)
    }
    ```

6. To try out the provider, place `pulumi-resource-myprovider` in PATH and create a new Pulumi YAML project to
   instantiate the provider's resources, and run `pulumi up` on that project:

   ```
   name: basicprogram
   runtime: yaml
   resources:
     r1:
       type: myprovider::MyResource
       properties:
         prop1: x
         prop2: y
   ```

7. If you want to test using the provider from other languages such as TypeScript, you can generate the SDKs for each
   language by running `pulumi-tfgen-myprovider` binary (see `--help` for all the options).


## How to Upgrade a Bridged Provider to Plugin Framework

Follow these steps if you have a Pulumi provider that was bridged from a Terraform provider built against [Terraform
Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) and you want to upgrade it to a version that has migrated
to the Plugin Framework.

1. Update `./provider/shim` by updating `go.mod` to point to a new version of the Terraform provider as you normally
   would for upstream upgrades. Make sure the module is now depending on
   `"github.com/hashicorp/terraform-plugin-framework"` instead of `"github.com/hashicorp/terraform-plugin-sdk/v2"`.

   Update the source code accordingly. For example, a `shim.go` that looked like this:

    ```go
    package shim

    import (
        "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
        "github.com/terraform-providers/terraform-provider-tls/internal/provider"
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
   and update it to call `tfgen.Main` from `github.com/pulumi/pulumi-terraform-bridge/pf/tfgen`.

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
   `github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge`. Note the signature changes: version parameter is removed,
   `Context` is now required, and there is a new `bridge-metadata.json` blob that needs to be embedded:

     ```go
     ...

     func main() {
         meta := tfbridge.ProviderMetadata{PackageSchema: schema}
         tfbridge.Main(context.Background(), "myprovider", myprovider.MyProvider(), meta)
     }
     ```

4. Update code declaring `tfbridge.ProviderInfo` (typically in `provider/resources.go`) from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge` and to declare `ProviderInfo` from
   `github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge` instead.

    ```go

    //go:embed cmd/pulumi-resource-myprovider/bridge-metadata.json
    var bridgeMetadata []byte

    func Provider() pf.ProviderInfo {
        info := tfbridge.ProviderInfo{
            // Comment out P, use NewProvider below instead.
            // P: shimv2.NewProvider(shim.NewProvider()),

            // Make sure Version is set, as it is now required.
            Version: ...,

            // This is now required.
            MetadataInfo: tfbridge.NewProviderMetadata(bridgeMetadata),

            // Keep the rest of the code as before.
        }
        return pf.ProviderInfo{
            ProviderInfo: info,
            NewProvider:  shim.NewProvider,
        }
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

## How to Upgrade a provider that has partially migrated to the Plugin Framework

Follow these steps if you have a Pulumi provider that was bridged from a Terraform provider built against [Terraform
Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) and you want to upgrade it to a version that has migrated
some but not all resources/datasources to the Plugin Framework.

1. Ensure you have access to the `github.com/hashicorp/terraform-plugin-framework/provider.Provider` from the upstream provider.
   If the provider is shimmed (or needs to be), you can follow step (1) from the section "How to Upgrade a Bridged Provider to
   Plugin Framework".

2. Find the tfgen binary `main` that calls `tfgen.Main` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen` and update
   it to call `tfgen.MainWithMuxer` from `github.com/pulumi/pulumi-terraform-bridge/pf/tfgen`.

   Note that the extra version parameter is removed from `tfgen.Main`, so this code:

   ```go
   tfgen.Main("cloudflare", version.Version, tls.Provider())
   ```

   Becomes:

   ``` go
   tfgen.MainWithMuxer("cloudflare", cloudflare.Provider())
   ```

3. Find the provider binary `main` that calls `tfbridge.Main` from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge` and update it to `MainWithMuxer` from
   `github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge`.

   Note the signature changes: version parameter is removed, and `Context` is now required, so this
   code:

     ```go
     tfbridge.Main("cloudflare", version.Version, cloudflare.Provider(), pulumiSchema)
     ```

     Becomes:

    ```go
    tfbridge.MainWithMuxer(context.Background(), "cloudflare", cloudflare.Provider(), pulumiSchema)
    ```

4. Update code declaring `tfbridge.ProviderInfo` (typically in `provider/resources.go`), changing the embedded
   `tfbridge.ProviderInfo.P` to the result of calling `github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge.MuxShimWithPF`.

   This function combines the original SDK based provider with a new PF based provider, so this code:

    ```go
    p := shimv2.NewProvider(provShim.NewProvider())

    ...

    prov := 	prov := tfbridge.ProviderInfo{
		P:                p,
        ...
    }
    ```

    Becomes:

    ```go
    p := pfbridge.MuxShimWithPF(context.Background(),
		shimv2.NewProvider(provShim.SDKProvider()),
		provShim.PFProvider(),
	)

    ...

    prov := 	prov := tfbridge.ProviderInfo{
		P:                p,
        ...
    }
    ```

> Muxed providers need an accompanying `MetadataInfo`. If the provider did not have one set up, this is the time to do so.

5. From this point the update proceeds as a typical upstream provider update. Build and run the tfgen binary to compute
   the Pulumi Package Schema. It will now also compute a new metadata file `bridge-metadata.json`, build the provider
   binary, re-generate language-specific SDKs and run tests.

    ```
    make tfgen
    make provider
    make build_sdks
    ```
