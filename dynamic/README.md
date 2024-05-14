# Dynamic Bridged Provider

A *dynamically bridged provider* is a Pulumi native provider parameterized by
the identity of a terraform provider. It consists of a binary
`pulumi-terraform-bridge`, which is spun up as a provider by `pulumi`. The
binary is responsible for downloading the terraform provider it is emulating,
then translating `pulumi`’s [gRPC
protocol](https://github.com/pulumi/pulumi/tree/master/proto/pulumi) into
[Terraform’s v6
protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol).
