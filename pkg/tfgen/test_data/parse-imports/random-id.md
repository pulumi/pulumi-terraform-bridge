Import is supported using the following syntax:

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
# Random IDs can be imported using the b64_url with an optional prefix. This
# can be used to replace a config value with a value interpolated from the
# random provider without experiencing diffs.

# Example with no prefix:
terraform import random_id.server p-9hUg

# Example with prefix (prefix is separated by a ,):
$ terraform import random_id.server my-prefix-,p-9hUg
```
