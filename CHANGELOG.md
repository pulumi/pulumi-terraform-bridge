CHANGELOG
=========

## HEAD (Unreleased)
* Add support for generating a .NET SDK.
* Update Terraform bridge to be based on v1.0.0 of the Terraform Plugin SDK
* Add support to specify a custom package name for NodeJS package
* Add ability to pass the TF provider version that the pulumi provider was generated against
* Remove the need for Pandoc in generating Python SDK readme files.
* Allow a schema variable to be overridden as being `Computed`
* Allow passing a License Type for the upstream Terraform provider.
* Warn when config with default values are not reflected in providers.
* Centralise the work for Autonaming in providers.
* Avoid setting conflicting default values ([91](https://github.com/pulumi/pulumi-terraform-bridge/pull/91)).
* Require explict C# namespaces.

---
