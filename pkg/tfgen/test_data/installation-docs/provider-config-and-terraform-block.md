This example declares nothing besides provider config, so it should render only the Pulumi config

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

## Example With A Resource

This example also declares a resource, so it should still render a language chooser

```hcl
terraform {
  required_providers {
    simple = {
      source = "example/simple"
    }
  }
}

provider "simple" {
  user_name = "admin"
}

resource "simple_resource" "a_resource" {
  input_one = "hello"
}
```

## Configuration Reference

The following configuration inputs are supported:
