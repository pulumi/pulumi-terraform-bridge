# `@pulumi/terraform` CHANGELOG

This CHANGELOG details important changes made in each version of the
`terraform` provider, and the `@pulumi/terraform` Node.js package.

## v0.18.4 (Unreleased)

- Terraform-based providers can now communicate detailed information about the difference between a resource's desired and actual state during a Pulumi update.
- Add the ability to inject CustomTimeouts into the InstanceDiff during a pulumi update.
- Better error message for missing required fields with default config ([#400](https://github.com/pulumi/pulumi-terraform/issues/400)).
- Change how Tfgen deals with package classes that are named Index to make them index_.ts
- Protect against panic in provider Create with InstanceState Meta initialization
- Use of the `RemoteStateReference` resource no longer results in a panic if the configured remote state cannot be accessed.
- Allow a provider to depend on a specific version of TypeScript.
- Allow users to specify a specific provider version.
- Add the ability to deprecate resources and datasources.
- Emit an appropriate user warning when Pulumi binary not found in Python setup.py.
- Add support for suppressing differences between the desired and actual state of a resource via the `ignoreChanges` property.

## v0.18.3 (Released June 20, 2019)

- Fixed a bug that caused unnecessary changes if the first operation after upgrading a bridged provider was a `pulumi refresh`.
- Fixed a bug that caused maps with keys containing a '.' character to be incorrectly treated as containing nested maps when deserializing Terraform attributes.

### Improvements

- Automatically generate `isInstance` type guards for implementations of `Resource`.
- `TransformJSONDocument` now accepts arrays (in addition to maps).

## v0.18.2 (Released May 28th, 2019)

- Improved the package `README` file to reflect usage of the `@pulumi/terraform`
  package rather than the Terraform bridge.

## v0.18.1 (Released May 16th, 2019)

- Initial release of `@pulumi/terraform` with support for Node.js.
