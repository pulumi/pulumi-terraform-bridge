# testprovider_sdkv2

Defines a provider using `github.com/hashicorp/terraform-plugin-sdk/v2` that is dedicated to testing the features of
`pulumi-terraform-bridge`. Specifically the provider is crafted to support `ProgramTest` tests that exercise it together
with the latest Pulumi CLI. These tests verify that the bridged providers integrate correctly with the engine.
