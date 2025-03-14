The resource `random.RandomString` generates a random permutation of alphanumeric characters and optionally special characters.

This resource *does* use a cryptographic random number generator.

Historically this resource's intended usage has been ambiguous as the original example used it in a password. For backwards compatibility it will continue to exist. For unique ids please use random_id, for sensitive random values please use random_password.

## Example Usage

```terraform
resource "random_string" "random" {
  length           = 16
  special          = true
  override_special = "/@£$"
}
```

## Import

You can import external strings into your Pulumi programs as RandomString resources as follows:

```sh<break>
$ import random:index/randomString:RandomString newString myspecialdata
<break>```

This command will encode the `myspecialdata` token in Pulumi state and generate a code suggestion to
include a new RandomString resource in your Pulumi program. Include the suggested code and do a
`pulumi up`. Your data is now stored in Pulumi, and you can reference it in your Pulumi program as
`newString.result`.

If the data needs to be stored securily as a secret, consider using the RandomPassword resource
instead.

