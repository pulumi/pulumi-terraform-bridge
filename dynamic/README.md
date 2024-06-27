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

Diving deeper into how the repo is laid out, we see:

``` console
$ eza -T --classify=always -I 'test*' --git-ignore
./
├── go.mod
├── go.sum
├── info.go
├── internal/
│  └── shim/
│     ├── go.mod
│     ├── go.sum
│     ├── protov5/
│     │  ├── provider.go
│     │  └── translate/
│     │     └── tfplugin5.go
│     ├── protov6/
│     │  ├── provider.go
│     │  └── translate/
│     │     └── tfplugin6.go
│     └── run/
│        ├── loader.go
│        └── loader_test.go
├── main.go
├── Makefile
├── provider_test.go
├── README.md
└── version/
   └── version.go
```

The dynamic provider layer is by-design as simple and straight-forward as possible. Each package does one
thing only and there isn't that much code. As of time of writing, the entire `dynamic` folder is only 2288
lines of go code[^1]. I'll go through each package in turn.

[^1]: `loc --exclude '*._test.go'`

### `package main`

`package main` is responsible for launching a Pulumi provider and setting up the parameterize call. It does
this by calling [`pf/tfbridge.Main`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/pf@v0.38.0/tfbridge#Main), passing in an empty PF provider (from
[`pf/proto.Empty()`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/pf@v0.38.0/proto#Empty)). [`pf/tfbridge.ProviderMetadata`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/pf@v0.38.0/tfbridge#ProviderMetadata) allows overriding the `Parameterize` and
`GetSchema` call (and we override both).

When `Parameterize` is called, we launch the underlying Terraform provider via
`internal/shim/run.LocalProvider` or `internal/shim/run.NamedProvider` (downloading as necessary). Both
functions return a [`tfprotov6.ProviderServer`](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-go/tfprotov6#ProviderServer) which is used to re-initialize the running provider via
[`pf/tfbridge.XParameterizeResetProvider`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/pf@v0.38.0/tfbridge#XParameterizeResetProvider).

When `GetSchema` is called, it generates a schema from the currently equipped provider with
[`pkg/tfgen.GenerateSchemaWithOptions`](https://pkg.go.dev/github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen#GenerateSchemaWithOptions) and returns is. All type translation, documentation generation, etc
are done with standard bridge based functionality.

All other gRPC calls (`Create`, `Read`, `Update`, `Delete`, etc.) are handled normally by `pf`'s existing
server.

### `package version`

`version.version` is used as a link-time target to bake in the release version to the provider binary. This is
the same mechanism that Pulumi uses to embed versions in all of our binaries.

### `package run`

`run` defines a running provider for the purposes of `dynamic`.

``` go
type Provider interface {
	tfprotov6.ProviderServer
	io.Closer

	Name() string
	Version() string
}
```

`run` also defines functions to "run" the underlying TF provider:

- `run.NamedProvider` takes a provider definition like `("cloudfront/cloudfront", ">= 3.2.0")` and loads the
  provider (downloading it if necessary). Named Terraform providers are cached in
  `PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR` (defaulting to `$PULUMI_HOME/dynamic_tf_plugins`).

- `run.LocalProvider` takes a path to a Terraform provider and runs it.

When `run` launches a Terraform provider, the provider may implement either the
[`tfplugin5.ProviderClient`](https://pkg.go.dev/github.com/opentofu/opentofu/internal/tfplugin5#ProviderClient) or [`tfplugin6.ProviderClient`](https://pkg.go.dev/github.com/opentofu/opentofu/internal/tfplugin6#ProviderClient) interface. `run` must return a
[`tfprotov6.ProviderServer`](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-go/tfprotov6#ProviderServer). The Terraform ecosystem helps with [translating from v5 to v6](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-mux/tf5to6server#UpgradeServer):

``` go
func tf5to6server.UpgradeServer(context.Context, func() tfprotov5.ProviderServer) (tfprotov6.ProviderServer, error)
```

We still need to be able to translate from [`tfplugin5.ProviderClient`](https://pkg.go.dev/github.com/opentofu/opentofu/internal/tfplugin5#ProviderClient) and [`tfplugin6.ProviderClient`](https://pkg.go.dev/github.com/opentofu/opentofu/internal/tfplugin6#ProviderClient)
to [`tfprotov5.ProviderServer`](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-go/tfprotov5#ProviderServer) and [`tfprotov6.ProviderServer`](https://pkg.go.dev/github.com/hashicorp/terraform-plugin-go/tfprotov6#ProviderServer) respectively. For that, see the next
section.

### `package protov5` & `package protov6`

`package protov5` and `package protov6` are nearly identical packages that translate between gRPC level client
types to just above gRPC level server types. Both packages are identical in structure, exposing one end point:

``` go
func New(tfplugin5.ProviderClient) tfprotov5.ProviderServer

func New(tfplugin6.ProviderClient) tfprotov6.ProviderServer
```

Both packages delegate type conversions to a `translate` sub-package, restricting themselves to fielding gRPC
calls.

A representative gRPC handler looks like this:

``` go
// tfprotov6/provider.go
import (
	"github.com/opentofu/opentofu/internal/tfplugin6"
	"github.com/opentofu/opentofu/shim/protov6/translate"
)

...

func (p shimProvider) ReadResource(
	ctx context.Context, req *tfprotov6.ReadResourceRequest,
) (*tfprotov6.ReadResourceResponse, error) {
	return translateGRPC(ctx,
		p.remote.ReadResource,
		translate.ReadResourceRequest(req),
		translate.ReadResourceResponse)
}
```

The `translate.ReadResourceRequest` call looks like this:

``` go
// tfprotov6/translate/tfplugin6.go
import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/opentofu/opentofu/internal/tfplugin6"
)

...

func ReadResourceRequest(i *tfprotov6.ReadResourceRequest) *tfplugin6.ReadResource_Request {
	if i == nil {
		return nil
	}

	return &tfplugin6.ReadResource_Request{
		TypeName:     i.TypeName,
		CurrentState: dynamicValueRequest(i.CurrentState),
		Private:      i.Private,
		ProviderMeta: dynamicValueRequest(i.ProviderMeta),
	}
}
```
