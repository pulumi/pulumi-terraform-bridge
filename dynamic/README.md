<!-- -*- fill-column: 110 -*- -->
# Dynamic Bridged Provider

A *dynamically bridged provider* is a Pulumi native provider parameterized by the identity of a terraform
provider. It consists of a binary `pulumi-terraform-bridge`, which is spun up as a provider by `pulumi`. The
binary is responsible for downloading the terraform provider it is emulating, then translating `pulumi`’s
[gRPC protocol](https://github.com/pulumi/pulumi/tree/master/proto/pulumi) into [Terraform’s v6
protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol).

## Usage

Dynamic bridged providers are a subset of parameterized providers <!-- TODO: Insert link to docs / blog post
if/when it exists -->, so they are used via parameterization. If you are using a language besides Pulumi YAML,
you start by generating an SDK.

### SDK Generation

SDK generation relies on an existing terraform provider. The Terraform provider can be in a Terraform registry
(such as [OpenTofu's Registry](https://opentofu.org/docs/internals/provider-registry-protocol/)) or local to
your machine.

#### Registry based SDK generation

To generate an SDK based on a Terraform provider in a Terraform Registry, use:

``` sh
pulumi package gen-sdk terraform-bridge [hostname/][namespace/]<type> [version] [--language <lang>]
```

For example, to generate a Typescript SDK for [Azure's Alz
provider](https://github.com/Azure/terraform-provider-alz) at version v0.11.1, you would run:

``` sh
pulumi package gen-sdk --language typescript terraform-bridge registry.opentofu.org/Azure/alz 0.11.1
```

At the time of writing, the latest version is `v0.11.1`, so you could drop the version:

``` patch
-pulumi package gen-sdk --language typescript terraform-bridge registry.opentofu.org/Azure/alz 0.11.1
+pulumi package gen-sdk --language typescript terraform-bridge registry.opentofu.org/Azure/alz
```

If no version is specified, then the latest version is used.

The default registry is `registry.opentofu.org`, so you can omit the registry as well:


``` patch
-pulumi package gen-sdk --language typescript terraform-bridge registry.opentofu.org/Azure/alz
+pulumi package gen-sdk --language typescript terraform-bridge Azure/alz
```

The information you entered (with the registry and the version specified) are embedded in the generated SDK,
so you won't need to enter any of this information again as long as you use the SDK that you generated.

#### Path based SDK generation


To generate an SDK based on a Terraform provider on your local file system, use:

``` sh
pulumi package gen-sdk terraform-bridge [path/]terraform-provider-[name]
```

The name of the provider must start with `terraform-provider-`.

## Architecture

The `pulumi-terraform-bridge` provider works by acquiring and running a Terraform provider, and then acting as
a translation middleware between the Pulumi engine and the Terraform provider.

A typical usage looks like this:

``` mermaid
sequenceDiagram
    participant P as pulumi
    create participant B as pulumi-terraform-bridge
    P->>B: Run Pulumi Provider
    P->>B: Parameterize({name: "example", version: "v1.2.3"})
    create participant T as terraform-provider-example
    B->>T: Run Terraform Provider

    P->>B: CreateRequest({type: "example:index:Example", props: {propertyValue: V}})
    B->>T: PlanResourceChangeRequest({type: "example_example", olds: {}, news: {property_value: V}})
    T->>B: PlanResourceChangeResult({type: "example_example", plan: {property_value: V'}})
    B->>T: ApplyResourceRequest({type: "example_example", plan: {property_value: V'}})
    T->>B: ApplyResourceResult({type: "example_example", plan: {property_value: V''}})
    B->>P: CreateResponse({type: "example:index:Example", props: {propertyValue: V''}})

    P->>B: Cancel
    B->>T: Shutdown
    destroy T
    T-->>B: Shutdown Complete
    destroy B
    B-->>P: Cancel done
```
