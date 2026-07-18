# API Design for Bridged Provider Packages

The following is a working list of design guidelines for packages built on top of the
`pulumi-terraform` projection of Terraform providers into Pulumi.

**Do** choose an official reference point for adding namespace structure on top of the TF
resources.

**Do** add `AltTypes` for strongly typed references to other resource kinds. This should be
done when the value of a property is the same as the `.id` property of another resource.

**Do not** change the `Type` when an `AltType` would suffice; prefer to leave the primary
type as the raw projection.

**Do** add enums for any enumerations defined in the AWS SDK. These are not explicitly
projected by Terraform (they are checked dynamically). These enums should overwrite the
`Type` of the property if they are confidently the full set of legal values.

**Do** add JSON schema for JSON-valued inputs in the AWS SDK. These are projected as strings
by Terraform. For now we will continue to project as string, and require users to
`JSON.stringify`, but we can provide strong types for the JSON schema as guides. (NOTE: we
may add support to [auto-`JSON.stringify`](https://github.com/pulumi/pulumi-terraform/issues/129)
in the future as an optional thing.)
