[![Build Status](https://travis-ci.com/pulumi/pulumi-terraform-bridge.svg?token=cTUUEgrxaTEGyecqJpDn&branch=master)](https://travis-ci.com/pulumi/pulumi-terraform-bridge)
[![Go Report Card](https://goreportcard.com/badge/github.com/pulumi/pulumi-terraform-bridge)](https://goreportcard.com/report/github.com/pulumi/pulumi-terraform-bridge)
[![GoDoc](https://godoc.org/github.com/pulumi/pulumi-terraform-bridge?status.svg)](https://godoc.org/github.com/pulumi/pulumi-terraform-bridge)

# Pulumi Terraform Bridge

This bridge adapts any [Terraform Provider](https://github.com/terraform-providers) built using the [Terraform Plugin
SDK](https://github.com/hashicorp/terraform-plugin-sdk) for use with Pulumi.  The Terraform
community provides resource providers that perform create, read, update, and delete (CRUD) operations for a broad array
of infrastructure providers and types.  In principle, any of them can be programmed using Pulumi with this bridge.

If you want to wrap a _new_ Terraform provider as a Pulumi provider, check out [pulumi/pulumi-tf-provider-boilerplate](https://github.com/pulumi/pulumi-tf-provider-boilerplate).

## Overview

Although the Terraform schema is used as a starting point, the concept of "overlays" enables customization, including
classification into modules, stronger typing, better documentation, and more.  Pulumi can also augment providers with
non-CRUD operations like queries, metrics, and logs -- while not having to repeat all of the considerable and quality
work that has already gone into building reliable CRUD operations against the major cloud providers' platforms.

Most users of Pulumi don't need to know how this bridge works.  Many will find it interesting, and, if you'd like to
bring up a new provider that is available in Terraform but not yet Pulumi, we would love to hear from you.

## How It Works

There are two major things involved in this bridge: design-time and runtime.

At design-time, we code-generate packages by dynamic inspection of a Terraform Provider's schema.  This only works for
providers that are built using static schemas.  It is possible to write Terraform Providers without this, which means
the ability to create packages would not exist, but in practice all interesting providers use it.

Second, the bridge connects the Pulumi engine to a given Terraform Provider using Pulumi's RPC interfaces.  This
behavior also leverages the Terraform provider schema, for operations like performing validation and diffs.

## Development

This section only matters if you want to build this bridge from scratch, or use it in your own project.

### Prerequisites

Before doing any development, there are a few prerequisites to install:

* Go: https://golang.org/dl
* [GolangCI-Lint](https://github.com/golangci/golangci-lint): `go get -u github.com/golangci/golangci-lint/cmd/golangci-lint`

### Building and Testing

There is a `Makefile` in the root that builds and tests everything.

To build, ensure `$GOPATH` is set, and clone into a standard Go workspace:

    $ git clone git@github.com:pulumi/pulumi-terraform-bridge $GOPATH/src/github.com/pulumi/pulumi-terraform-bridge
    $ cd $GOPATH/src/github.com/pulumi/pulumi-terraform-bridge

You can run make to build and run tests:

    $ make

This repo on its own isn't particularly interesting, until it is used to create a new Pulumi provider.

### Releasing

We use git tags and [GitHub Releases](https://github.com/pulumi/pulumi-terraform-bridge/releases).
Maintainers will push a new semver tag when appropriate.

### Adapting a New Terraform Provider

It is relatively easy to adapt a Terraform Provider, X, for use with Pulumi.  The
[AWS provider](https://github.com/pulumi/pulumi-aws) offers a good blueprint for how to go about this.

You will create two Go binaries -- one purely for design-time usage to act as X's code-generator and the other for
runtime usage to serve as its dynamic resource plugin -- and link with the Terraform Provider repo and this one.
There is then typically a `resources.go` file that maps all of the Terraform Provider metadata available at runtime
to types and concepts that the bridge will use to generate well-typed programmatic abstractions.

The AWS provider provides a standard blueprint to follow for this.  There are three major elements:

* [`cmd/pulumi-tfgen-aws/`](https://github.com/pulumi/pulumi-aws/tree/master/provider/cmd/pulumi-tfgen-aws)
* [`cmd/pulumi-resource-aws/`](https://github.com/pulumi/pulumi-aws/tree/master/provider/cmd/pulumi-resource-aws)
* [`resources.go`](https://github.com/pulumi/pulumi-aws/blob/master/provider/resources.go)

The [`Makefile`](https://github.com/pulumi/pulumi-aws/blob/master/Makefile) compiles these programs, and notably, uses
the resulting `pulumi-tfgen-aws` binary to generate code for many different languages.  The resulting generated code is
stored in the [`sdk` directory](https://github.com/pulumi/pulumi-aws/tree/master/sdk).

### Augmenting Auto-Generated Code w/ Overlays

An overlay is a set of additional directives that the code generator obeys when creating the final packages.

These may specify additional types, functions, or entire modules in this directory may be merged into the resulting
package.  This can be useful for helper modules and functions, in addition to gradual typing, such as using strongly
typed enums in places where the underlying provider may only have weakly typed strings.

To do this, first add the files in the appropriate package sub-directory of the sdk, and then add the requisite directives to the
provider file.  See the [AWS overlays section in resources.go](https://github.com/pulumi/pulumi-aws/blob/master/provider/resources.go#L4486) for
an example of this in action.

# tfgen options

tfgen, the command that generates Pulumi schema/code for a bridged provider supports the following environment variables:

* `PULUMI_SKIP_MISSING_MAPPING_ERROR`: If truthy, tfgen will not fail if a data source or resource in the TF provider is not mapped to the Pulumi provider. Instead, a warning is printed. Default is `false`.
* `PULUMI_SKIP_EXTRA_MAPPING_ERROR`: If truthy, tfgen will not fail if a mapped data source or resource does not exist in the TF provider. Instead, warning is printed. Default is `false`.
* `PULUMI_MISSING_DOCS_ERROR`: If truthy, tfgen will fail if docs cannot be found for a data source or resource. Default is `false`.
