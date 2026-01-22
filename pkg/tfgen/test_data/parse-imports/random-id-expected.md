## Import

Random IDs can be imported using the b64_url with an optional prefix. This

can be used to replace a config value with a value interpolated from the

random provider without experiencing diffs.

Example with no prefix:

```sh
$ pulumi import random:index/id:Id server p-9hUg
```

Example with prefix (prefix is separated by a ,):

```sh
$ pulumi import random:index/id:Id server my-prefix-,p-9hUg
```

