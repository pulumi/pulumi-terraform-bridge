# `@pulumi/terraform` CHANGELOG

This CHANGELOG details important changes made in each version of the
`terraform` provider, the `@pulumi/terraform` Node.js package and the
`pulumi_terraform` Python package.

## v0.18.4 (Unreleased)

- Initial release of `pulumi_terraform` for Python.
- `RemoteStateReference` resources can now read states created with Terraform 0.12.6 and below.
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
- Fix a bug that caused the recalculation of defaults for values that are normalized in resource state.
- The Python SDK generated for a provider now supports synchronous invokes.
- The Python SDK generated for a provider now supports calling `SomeResource.get(...)` to create a
  resource with the state of existing cloud resource.

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
