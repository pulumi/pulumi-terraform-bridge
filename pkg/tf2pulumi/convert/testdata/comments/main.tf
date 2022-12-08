// Check we keep variable comments
variable "opt_str_in" {
  // About the default
  default = "some string"
  // About the type
  type = string // More type comments
}

// Check we keep local comments
locals {
    // About the bool local
    a_bool = true
    // Trailing bool comment
}

// Check we keep data source comments
data "complex_data_source" "a_data_source" {
    // About properties
    a_bool = local.a_bool
    a_number = 2.3 // Trailing comments on properties
    a_string = var.opt_str_in
    a_list_of_int = [1, 2, 3]
    a_map_of_bool = {
        // In maps
        a: true
        b: false
    }
    inner_object {
        // In objects
        inner_string = "hello again"
    }
}

// Check that we keep resource comments
resource "complex_resource" "a_resource" {
    // About properties
    a_bool = true
    a_number = 2.3 // Trailing comments on properties
    a_string = "hello world"
    a_list_of_int = [1, 2, 3]
    a_map_of_bool = {
        // In maps
        a: true
        b: false
    }
    inner_object {
        // In objects
        inner_string = "hello again"
    }
}

// Check that we keep output comments
output "some_output" {
    // About the output value
    value = complex_resource.a_resource.result
}