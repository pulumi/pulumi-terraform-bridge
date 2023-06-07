resource "simple_resource" "example" {
    input_one = data.unknown_data_source.example.attr
    input_two = unknown_resource_type.example.list[0]
}

resource "simple_resource" "another_example" {
    input_one = example_resource_type.resource_name.some_attribute
    input_two = var.some_config.test_attribute
}

output "test_unknown_already_declared_data_source" {
    value = data.unknown_data_source.example.another_test_attribute
}

output "test_unknown_local_variable" {
    value = local.some_variable_name
}

output "test_unknown_conflicting_local_variable" {
    value = local.example.another_test_attribute
}