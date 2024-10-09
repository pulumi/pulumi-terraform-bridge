# Cross-tests for PF

This package provides [cross-testing](../../../../pkg/tests/cross-tests/README.md) for [Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework) based Terraform
providers, bridged into Pulumi with [pf](../../../README.md).

It *does not* contain cross-tests. It just provides a library for writing cross-tests.

An example usage looks like this:

``` go
func TestConfigure(t *testing.T) {
	t.Parallel()

    schema := schema.Schema{Attributes: map[string]schema.Attribute{
		"k": schema.StringAttribute{Optional: true},
	}}

    tfInput := map[string]cty.Value{"k": cty.StringVal("foo")}

    puInput := resource.PropertyMap{"k": resource.MakeSecret(resource.NewProperty("foo"))}

	crosstests.Configure(schema, tfInput, puInput)
}
```

Here, the cross-test will assert that a provider whose configuration is described by
`schema` will observe the same inputs when configured in via HCL with the inputs
`tfInputs` and when bridged and configured with Pulumi and `puInputs`.

The idea is that the "Configured Provider" should not be able to tell if it was configured
via HCL or Pulumi YAML:


```
    +--------------------+                      +---------------------+
    | Terraform Provider |--------------------->| Configure(tfInputs) |
    +--------------------+                      +---------------------+
              |                                                        \
              |                                                         \
              |                                                          \
              |                                                      +---------------------+
              | tfbridge.ShimProvider                                | Configured Provider |
              |                                                      +---------------------+
              |                                                          /
              |                                                         /
              V                                                        /
    +--------------------+                      +---------------------+
    |   Pulumi Provider  |--------------------->| Configure(puInputs) |
    +--------------------+                      +---------------------+
```

