This example is invalid and should not be translated for this test to pass

## Example Usage

```hcl
# Configure the OpenStack Provider
misspelledprovider "simple-provider" {
  user_name   = "admin"
  tenant_name = "admin"
  password    = "pwd"
  auth_url    = "http://myauthurl:5000/v3"
  region      = "RegionOne"
}
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
