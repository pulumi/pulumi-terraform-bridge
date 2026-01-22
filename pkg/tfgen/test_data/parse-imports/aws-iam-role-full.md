## Import

In Terraform v1.12.0 and later, the `import` block can be used with the `identity` attribute. For example:

```terraform
import {
  to = aws_iam_role.example
  identity = {
    name = "developer_name"
  }
}

resource "aws_iam_role" "example" {
  ### Configuration omitted for brevity ###
}
```

### Identity Schema

#### Required

* `name` (String) Name of the IAM role.

#### Optional

* `account_id` (String) AWS Account where this resource is managed.

In Terraform v1.5.0 and later, use an `import` block to import IAM Roles using the `name`. For example:

```terraform
import {
  to = aws_iam_role.example
  id = "developer_name"
}
```

Using `terraform import`, import IAM Roles using the `name`. For example:

```console
% terraform import aws_iam_role.example developer_name
```
