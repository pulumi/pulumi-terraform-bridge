# Developing a New Plugin Framework Provider

Follow these steps to bridge a Terraform Provider to Pulumi.

1. You will need a Provider value from the `github.com/hashicorp/terraform-plugin-framework/provider`
   package. You can build it yourself as part of developing a Terraform Provider, or find it in published
   Terraform sources.

   For example, `terraform-provider-random` [exposes](https://github.com/hashicorp/terraform-provider-random/blob/main/internal/provider/provider.go#L13) a `func New() provider.Provider` call. Since this
   definition lives in an `internal` package it cannot easily be referenced in an external Go project, but it
   is still possible to reference it using Go linker tricks. See [pulumi-random](https://github.com/pulumi/pulumi-random/tree/48c0b3014aeaa0cb95fd6419d631cb2555ce89ac/provider/shim) for a full example.

2. Populate a `ProviderInfo` struct, mapping Terraform resource names to Pulumi tokens. Replace `myprovider`
   with your provider name.

    ```go
    package myprovider

    import (
        _ "embed"
        "github.com/hashicorp/terraform-plugin-framework/provider"
        pf "github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge"
        "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
    )

    //go:embed cmd/pulumi-resource-myprovider/bridge-metadata.json
    var bridgeMetadata []byte

    func MyProvider() tfbridge.ProviderInfo {
        info := tfbridge.ProviderInfo{
            P:       pf.ShimProvider(<TODO fill in Terraform Provider from Step 1>),
            Name:    "myprovider",
            Version: "1.2.3",
            Resources: map[string]*tfbridge.ResourceInfo{
                "myresource": {Tok: "myprovider::MyResource"},
            },
            MetadataInfo: tfbridge.NewProviderMetadata(bridgeMetadata),
        }
        return info
    }
    ```

3. Build a `pulumi-tfgen-myprovider` binary.

    ```go
    package main

    import (
        "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
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

        "github.com/pulumi/pulumi-terraform-bridge/v3/pf/tfbridge"
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

7. If you want to test using the provider from other languages such as TypeScript, you can generate the SDKs
   for each language by running `pulumi-tfgen-myprovider` binary (see `--help` for all the options).
