## Import

In Terraform v1.12.0 and later, the `import` block can be used with the `identity` attribute. For example:


### Identity Schema

#### Required

* `name` (String) Name of the IAM role.

#### Optional

* `account_id` (String) AWS Account where this resource is managed.

In Terraform v1.5.0 and later, use an `import` block to import IAM Roles using the `name`. For example:


Using `terraform import`, import IAM Roles using the `name`. For example:

```sh
$ pulumi import aws:iam/role:Role example developer_name
```

