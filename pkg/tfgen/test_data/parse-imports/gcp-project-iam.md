## Import

-> **Custom Roles** If you're importing a IAM resource with a custom role, make sure to use the
 full name of the custom role, e.g. `[projects/my-project|organizations/my-org]/roles/my-custom-role`.

-> **Conditional IAM Bindings**: If you're importing a IAM binding with a condition block, make sure
 to include the title of condition, e.g. `terraform import google_project_iam_binding.my_project "{{your-project-id}} roles/{{role_id}} condition-title"`
 
### Importing IAM members

IAM member imports use space-delimited identifiers that contain the resource's  `project_id`, `role`, and `member` e.g.

* `"{{project_id}} roles/viewer user:foo@example.com"`

An [`import` block](https://developer.hashicorp.com/terraform/language/import) (Terraform v1.5.0 and later) can be used to import IAM members:

```tf
import {
  id = "{{project_id}} roles/viewer user:foo@example.com"
  to = google_project_iam_member.default
}
```

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can also be used:

```
$ terraform import google_project_iam_member.default "{{project_id}} roles/viewer user:foo@example.com"
```

#### Import via resource identity

`google_project_iam_member` also supports plannable import via [resource identity](https://developer.hashicorp.com/terraform/language/block/import#identity) (Terraform 1.12+):

```tf
import {
  to = google_project_iam_member.default
  identity = {
    project = "your-project-id"
    role    = "roles/viewer"
    member  = "user:foo@example.com"
  }
}
```

Identity attributes:

* `project` - (Optional) The project id. May be omitted if a default project is configured on the provider.
* `role` - (Required) The IAM role being granted.
* `member` - (Required) The identity that the role is granted to.
* `condition_title` - (Optional) Title of the IAM condition, when importing a conditional binding.

### Importing IAM bindings

IAM binding imports use space-delimited identifiers that contain the `org_id` and role, e.g.

* `"{{project_id}} roles/viewer"`

An [`import` block](https://developer.hashicorp.com/terraform/language/import) (Terraform v1.5.0 and later) can be used to import IAM bindings:

```tf
import {
  id = "{{project_id}} roles/viewer"
  to = google_project_iam_binding.default
}
```

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can also be used:

```
terraform import google_project_iam_binding.default "{{project_id}} roles/viewer"
```

#### Import via resource identity

`google_project_iam_binding` also supports plannable import via [resource identity](https://developer.hashicorp.com/terraform/language/block/import#identity) (Terraform 1.12+):

```tf
import {
  to = google_project_iam_binding.default
  identity = {
    project = "your-project-id"
    role    = "roles/viewer"
  }
}
```

Identity attributes:

* `project` - (Optional) The project id. May be omitted if a default project is configured on the provider.
* `role` - (Required) The IAM role being granted.
* `condition_title` - (Optional) Title of the IAM condition, when importing a conditional binding.

### Importing IAM policies

IAM policy imports use the identifier of the Project only. For example:

* `"{{project_id}}"`

An [`import` block](https://developer.hashicorp.com/terraform/language/import) (Terraform v1.5.0 and later) can be used to import IAM policies:

```tf
import {
  id = "{{project_id}}"
  to = google_project_iam_policy.default
}
```

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can also be used:

```
$ terraform import google_project_iam_policy.default {{project_id}}
```

#### Import via resource identity

`google_project_iam_policy` also supports plannable import via [resource identity](https://developer.hashicorp.com/terraform/language/block/import#identity) (Terraform 1.12+):

```tf
import {
  to = google_project_iam_policy.default
  identity = {
    project = "your-project-id"
  }
}
```

Identity attributes:

* `project` - (Optional) The project id. May be omitted if a default project is configured on the provider.

### Importing Audit Configs

An audit config can be imported into a `google_project_iam_audit_config` resource using the resource's `project_id` and the `service`, e.g:

* `"{{project_id}} foo.googleapis.com"`


An [`import` block](https://developer.hashicorp.com/terraform/language/import) (Terraform v1.5.0 and later) can be used to import audit configs:

```tf
import {
  id = "{{project_id}} foo.googleapis.com"
  to = google_project_iam_audit_config.default
}
```

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can also be used:

```
terraform import google_project_iam_audit_config.default "{{project_id}} foo.googleapis.com"
```
