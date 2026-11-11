This example should render only the Pulumi config commands

## Example Usage

```hcl
terraform {
  required_providers {
    simple = {
      source = "example/simple"
    }
  }
}

# Configure the Simple Provider
provider "simple" {
  user_name = "admin"
  password  = "pwd"
}
```

## Configuration Reference

The following configuration inputs are supported:
