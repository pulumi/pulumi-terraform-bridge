## Import

### Identity Schema

#### Required

* `name` (String) Name of the IAM role.

#### Optional

* `account_id` (String) AWS Account where this resource is managed.


Using `pulumi import`, import IAM Roles using the `name`. For example:

```sh
$ pulumi import aws:iam/role:Role example developer_name
```

