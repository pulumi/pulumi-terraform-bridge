Use the navigation to the left to read about the available resources.

## Example Usage

```hcl
# Define required providers
terraform {
  required_version = ">= 0.14.0"
  required_providers {
    simple = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 1.53.0"
    }
  }
}

# Configure the OpenStack Provider
provider "openstack" {
  user_name   = "admin"
  tenant_name = "admin"
  password    = "pwd"
  auth_url    = "http://myauthurl:5000/v3"
  region      = "RegionOne"
}
## Define a resource
resource "openstack_resource" "a_resource" {
  input_one = "hello"
  input_two = true
}

output "some_output" {
  value = openstack_resource.a_resource.result
}
```

## Configuration Reference
The following configuration inputs are supported:
