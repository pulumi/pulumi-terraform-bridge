This example will only translate the resource code. It has no configuration file.

## Example Usage

```hcl
## Define a resource
resource "simple_resource" "a_resource" {
  input_one = "hello"
  input_two = true
}

output "some_output" {
  value = simple_resource.a_resource.result
}
```

## Configuration Reference

The following configuration inputs are supported:
