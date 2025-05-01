This example should translate at least the Pulumi config

## Example Usage

```hcl
# Configure the Simple Provider
provider "simple" {
  user_name   = "admin"
  tenant_name = "admin"
  password    = "pwd"
  auth_url    = "http://myauthurl:5000/v3"
  region      = "RegionOne"
}
```

## Configuration Reference

The following configuration inputs are supported:
