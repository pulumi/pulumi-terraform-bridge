# Sets in Terraform

Terraform supports Sets as a special collection type where two collections are considered equal up to element
reordering. In particular this means that Terraform considers set element reordering in programs a no-change plan.

## Sets in SDKv2 providers

To decide if two set elements are equal, TF consults the provider to compute an integer controlled by the provider
author with
[SchemaSetFunc](https://github.com/pulumi/terraform-plugin-sdk/blob/upstream-v2.33.0/helper/schema/schema.go#L246).
Notably this integer is not written to state but is ephemeral.

## Sets in Plugin Framework

The [Set](https://developer.hashicorp.com/terraform/plugin/framework/handling-data/types/set) collection is supported as
with SDKv2 but SchemaSetFunc is not available for Plugin Framework authors. However, similar functionality can be
achieved by using custom-typed values that may override
[Equal](https://github.com/hashicorp/terraform-plugin-framework/blob/v1.10.0/attr/value.go#L55) to define a custom
notion of equality.

Per [Migration Notes](https://developer.hashicorp.com/terraform/plugin/framework/migrating/schema#migration-notes):

> In SDKv2, schema structs have a Set field which can be populated with a SchemaSetFunc which is used for hashing. In
> the Framework, this is not required and does not need to be migrated.

## Example

In the following example, grants to the aws_s3_bucket_acl resource are set elements. Reordering them does not result in
any detected changes when using Terraform CLI.

``` hcl
    data "aws_canonical_user_id" "current" {}

    resource "aws_s3_bucket" "example" {
      bucket = "my-tf-example-bucket-t0yv0-2024"
    }

    resource "aws_s3_bucket_ownership_controls" "example" {
      bucket = aws_s3_bucket.example.id
      rule {
        object_ownership = "BucketOwnerPreferred"
      }
    }

    resource "aws_s3_bucket_acl" "example" {
      depends_on = [aws_s3_bucket_ownership_controls.example]

      bucket = aws_s3_bucket.example.id
      access_control_policy {
        grant {
          grantee {
            type = "Group"
            uri  = "http://acs.amazonaws.com/groups/s3/LogDelivery"
          }
          permission = "READ_ACP"
        }

        grant {
          grantee {
            id   = data.aws_canonical_user_id.current.id
            type = "CanonicalUser"
          }
          permission = "READ"
        }

        owner {
          id = data.aws_canonical_user_id.current.id
        }
      }
    }
```
