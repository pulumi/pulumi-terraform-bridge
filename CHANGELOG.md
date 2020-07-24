CHANGELOG
=========

## HEAD (Unreleased)

- Updating regular expressions for unordered lists in markdown
  [#234](https://github.com/pulumi/pulumi-terraform-bridge/pull/234) 
  
- Clean codegen directories before generating new code.
  [#236](https://github.com/pulumi/pulumi-terraform-bridge/pull/236)

- Convert examples using the current schema.
  [#224](https://github.com/pulumi/pulumi-terraform-bridge/pull/224)
  
- Add check against empty assets.
  [#223](https://github.com/pulumi/pulumi-terraform-bridge/pull/223)

- Add support for generating Go examples.
  [#194](https://github.com/pulumi/pulumi-terraform-bridge/pull/218)
  
- Add JSON path links to properties when generating doc comments and deprecation messages for
  a Pulumi schema.
  [#202](https://github.com/pulumi/pulumi-terraform-bridge/pull/202)

- Update README generation for python. 
  [#217](https://github.com/pulumi/pulumi-terraform-bridge/pull/217)

- Use the schema-based code generator for NodeJS.
  [#194](https://github.com/pulumi/pulumi-terraform-bridge/pull/194)

- Allow providers to define additional object types in the schema.
  [#192](https://github.com/pulumi/pulumi-terraform-bridge/pull/192)

- Populate default TF timeouts.
  [#177](https://github.com/pulumi/pulumi-terraform-bridge/issues/177)

- Link in tf2pulumi for preliminary HCL2 support and Python codegen.
  [#162](https://github.com/pulumi/pulumi-terraform-bridge/pull/162)

- Update how description is populated for schema codegen.
  [#148](https://github.com/pulumi/pulumi-terraform-bridge/pull/148))

- Use the schema-based code generator for Python.
  [#108](https://github.com/pulumi/pulumi-terraform-bridge/pull/108)

- Obtain module documentation from the Go Module Cache, removing the need for vendoring.

- Add support for generating a new-style Go SDK.
  [#101](https://github.com/pulumi/pulumi-terraform-bridge/pull/101)

- Add support for PEP 561 Python type specs. 
  [100](https://github.com/pulumi/pulumi-terraform-bridge/pull/100)

- Add support for generating a .NET SDK.

- Update Terraform bridge to be based on v1.0.0 of the Terraform Plugin SDK

- Add support to specify a custom package name for NodeJS package

- Add ability to pass the TF provider version that the pulumi provider was generated against

- Remove the need for Pandoc in generating Python SDK readme files.

- Allow a schema variable to be overridden as being `Computed`

- Allow passing a License Type for the upstream Terraform provider.

- Warn when config with default values are not reflected in providers.

- Centralise the work for Autonaming in providers.

- Avoid setting conflicting default values.
  [#91](https://github.com/pulumi/pulumi-terraform-bridge/pull/91)

- Require explict C# namespaces.

- Add option to control if only asynchronous data sources should be generated in JS/TS.

- Ensure Terraform deprecations are represented in Pulumi schema

- Ensure links to Terraform documentation pages are valid

- Prefer errors over panics for potential upstream error catches

- Ensure Pulumi SchemaInfo is taken into consideration when pluralizing parameters

- Add a version flag to providers
  [154](https://github.com/pulumi/pulumi-terraform-bridge/pull/91)

---
