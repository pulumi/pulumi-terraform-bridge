CHANGELOG
=========

## HEAD (Unreleased)
* Use the schema-based code generator for Python ([108](https://github.com/pulumi/pulumi-terraform-bridge/pull/108)).
* Obtain module documentation from the Go Module Cache, removing the need for vendoring.
* Add support for generating a new-style Go SDK ([101](https://github.com/pulumi/pulumi-terraform-bridge/pull/101)).
* Add support for PEP 561 Python type specs ([100](https://github.com/pulumi/pulumi-terraform-bridge/pull/100)).
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
* Add option to control if only asynchronous data sources should be generated in JS/TS.

---
