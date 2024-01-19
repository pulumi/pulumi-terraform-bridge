# Developing a New Provider

It is relatively easy to adapt a Terraform Provider, X, for use with Pulumi.  The
[Cloudflare provider](https://github.com/pulumi/pulumi-cloudflare) offers a good blueprint for how to go about this.

You will create two Go binaries -- one purely for design-time usage to act as X's code-generator and the other for
runtime usage to serve as its dynamic resource plugin -- and link with the Terraform Provider repo and this one.
There is then typically a `resources.go` file that maps all of the Terraform Provider metadata available at runtime
to types and concepts that the bridge will use to generate well-typed programmatic abstractions.

The Cloudflare provider provides a standard blueprint to follow for this.  There are three major elements:

* [`cmd/pulumi-tfgen-cloudflare/`](https://github.com/pulumi/pulumi-cloudflare/tree/master/provider/cmd/pulumi-tfgen-cloudflare)
* [`cmd/pulumi-resource-cloudflare/`](https://github.com/pulumi/pulumi-cloudflare/tree/master/provider/cmd/pulumi-resource-cloudflare)
* [`resources.go`](https://github.com/pulumi/pulumi-cloudflare/blob/master/provider/resources.go)

The [`Makefile`](https://github.com/pulumi/pulumi-cloudflare/blob/master/Makefile) compiles these programs, and notably, uses
the resulting `pulumi-tfgen-cloudflare` binary to generate code for many different languages.  The resulting generated code is
stored in the [`sdk` directory](https://github.com/pulumi/pulumi-cloudflare/tree/master/sdk).


## Building `tfbridge.ProviderInfo`

The main interaction point between the bridged provider authors and the bridge itself if via [tfbridge.ProviderInfo](https://github.com/pulumi/pulumi-terraform-bridge/blob/5e17c6c7e2d877db7e1d9c0b953a06d3ecabbaea/pkg/tfbridge/info.go#L48). 


## Token Mapping

Each upstream Terraform resource needs to be given a Pulumi appropriate name, called a token. We call this process token mapping. A simple mapping looks like this:

```go
			"aws_s3_bucket": {
				Tok: tfbridge.MakeResource(awsPackage, s3Mod, "Bucket"),
			},
```
### Automatic Token Mapping

Mapping a couple of resources is fine, but it quickly becomes tiresome to provide a manual mapping for each resource and datasource in a large provider, especially since new updates to the provider introduce new resources and remove old resources. The solution is *automatic token mappings*. 

For example:

```go
prov.MustComputeTokens(tokens.SingleModule("docker_", "index",
	tokens.MakeStandard(dockerPkg)))
```

[source](https://github.com/pulumi/pulumi-docker/blob/014b3fa8b3d9369d4108e71006cf8d429c19bc13/provider/resources.go#L369-L371)

See [automatic token mapping](./automatic-token-mapping.md) for more information.

## Augmenting a Terraform Provider

To add new resources/datasoruces or replace existing resources/datasoruces to the bridge Terraform provider, see [MuxWith](./muxwith.md).
