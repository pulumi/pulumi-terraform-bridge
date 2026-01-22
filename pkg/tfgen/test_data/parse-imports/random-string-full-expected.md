## Import

```sh
$ pulumi import random:index/string:String test test
```

### Limitations of Import

Any attribute values that are specified within Terraform config will be
ignored during import and all attributes that have defaults defined within
the schema will have the default assigned.

For instance, using the following config during import:
```terraform
resource "random_string" "test" {
  length = 16
  lower  = false
}
```

Then importing the resource using `terraform import random_string.test test`,
would result in the triggering of a replacement (i.e., destroy-create) during
the next `pulumi up`.

### Avoiding Replacement

If the resource were imported using `terraform import random_string.test test`,
replacement can be avoided by using:

1. Attribute values that match the imported ID and defaults:
    ```terraform
    resource "random_string" "test" {
      length = 4
      lower  = true
    }
    ```

2. Attribute values that match the imported ID and omit the attributes with defaults:
    ```terraform
    resource "random_string" "test" {
      length = 4
    }
    ```

3. `ignore_changes` specifying the attributes to ignore:
    ```terraform
    resource "random_string" "test" {
      length = 16
      lower  = false

      lifecycle {
        ignore_changes = [
          length,
          lower,
        ]
      }
    }
    ```

    **NOTE** `ignore_changes` is only required until the resource is recreated after import,
    after which it will use the configuration values specified.

