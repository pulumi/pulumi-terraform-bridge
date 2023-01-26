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
        "github.com/hashicorp/terraform-plugin-framework/provider"
        "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge"
        "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
    )

    func MyProvider() tfpfbridge.ProviderInfo {
        info := tfbridge.ProviderInfo{
            Name:    "myprovider",
            Version: "1.2.3",
            Resources: map[string]*tfbridge.ResourceInfo{
                "myresource": {Tok: "myprovider::MyResource"},
            },
        }
        return tfpfbridge.ProviderInfo{
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
        "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/tfgen"
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

        tfbridge "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge"
        // import myprovider
    )

    //go:embed schema.json
    var schema []byte

    //go:embed bridge-metadata.json
    var bridgeMetadata []byte

    func main() {
        meta := tfbridge.ProviderMetadata{PackageSchema: schema, BridgeMetadata: bridgeMetadata}
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
to the Plugin Framework. An example of a provider in this situation is [Pulumi Random Provider
v4.8.2](https://github.com/pulumi/pulumi-random/tree/v4.8.2).

1. Find the code that produces a Provider value from the `github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema`
   package and update it to get a Provider value from the `github.com/hashicorp/terraform-plugin-framework/provider`
   package.

2. Find tfgen binary `main` that calls `tfgen.Main` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen`
   and update it to call `tfgen.Main` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfpfbridge/tfgen`.

3. Find the provider binary `main` that calls `tfbridge.Main` from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge` and update it to `bridge.Main` from
   `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfpfbridge`.

4. Find code declaring `tfbridge.ProviderInfo` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge` and
   update it to declare `info.ProviderInfo` from `github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfpfbridge/info`
   instead. Work through compilation errors by commenting out features that are not suppored.

5. Build the provider
