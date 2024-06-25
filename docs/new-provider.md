# Developing a New Provider

It is relatively easy to adapt a Terraform Provider, X, for use with Pulumi.  The
[Cloudflare provider](https://github.com/pulumi/pulumi-cloudflare) offers a good starting
point for creating a new bridged provider.

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

Each upstream Terraform resource needs to be given a Pulumi appropriate name, called a token. We call this process token
mapping. A simple mapping looks like this:

```go
			"aws_s3_bucket": {
				Tok: tfbridge.MakeResource(awsPackage, s3Mod, "Bucket"),
			},
```
### Automatic Token Mapping

Mapping a couple of resources is fine, but it quickly becomes tiresome to provide a manual mapping for each resource and
datasource in a large provider, especially since new updates to the provider introduce new resources and remove old resources. The
solution is *automatic token mappings*.

For example:

```go
prov.MustComputeTokens(tokens.SingleModule("docker_", "index",
	tokens.MakeStandard(dockerPkg)))
```

[source](https://github.com/pulumi/pulumi-docker/blob/014b3fa8b3d9369d4108e71006cf8d429c19bc13/provider/resources.go#L369-L371)

See [automatic token mapping](./automatic-token-mapping.md) for more information.

## Augmenting a Terraform Provider

To add new resources/datasources or replace existing resources/datasources to the bridge
Terraform provider, see [MuxWith](./muxwith.md).

## Pulumi Bridge for Terraform Plugin Framework

For instructions on bridging a Terraform provider built using the [Terraform Plugin
Framework](https://developer.hashicorp.com/terraform/plugin/framework), please see
[pf/README.md](../pf/README.md).

## Version Requirements

The `pulumi-terraform-bridge` depends on non-semver internals of [pulumi/pulumi](https://github.com/pulumi/pulumi) and on a fork
of [terraform-plugin-sdk/v2](https://github.com/hashicorp/terraform-plugin-sdk):
[github.com/pulumi/terraform-plugin-sdk/v2](https://github.com/pulumi/terraform-plugin-sdk). You will need to match the version of
both in your provider's `go.mod` file.

### pulumi/pulumi

Keeping pulumi/pulumi at the right version is as simple as not manually upgrading the version of `github.com/pulumi/pulumi/pkg/v3`
and `github.com/pulumi/pulumi/sdk/v3` required. When you upgrade your dependency on `pulumi-terraform-bridge`, it will pull in the
version it needs, upgrading as necessary. We try to keep the version that the bridge uses in sync with the latest version of
[pulumi/pulumi](https://github.com/pulumi/pulumi).

### [github.com/pulumi/terraform-plugin-sdk/v2](https://github.com/pulumi/terraform-plugin-sdk)

You will need to add a `replace` directive to your `provider/go.mod` file that matches the one found in
https://github.com/pulumi/pulumi-terraform-bridge/blob/master/go.mod.  We don't change the version of
[github.com/pulumi/terraform-plugin-sdk/v2](https://github.com/pulumi/terraform-plugin-sdk) very often, but when we do the bridge
generally won't compile on the wrong version. Best practice is to check the SHA of the `replace` in the bridge uses and ensure
that your `replace` matches. We know that this is annoying and would like to [remove the fork
requirement](https://github.com/pulumi/pulumi-terraform-bridge/issues/1956).
