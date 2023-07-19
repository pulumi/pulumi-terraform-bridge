resource example "simple:index:resource" {
    inputOne = data.unknown_data_source.example.attr
    inputTwo = unknown_resource_type.example.list[0]
}
resource anotherExample "simple:index:resource" {
    inputOne = example_resource_type.resource_name.some_attribute
    inputTwo = var.some_config.test_attribute
}
output testUnknownAlreadyDeclaredDataSource {
    value = data.unknown_data_source.example.another_test_attribute
}
output testUnknownLocalVariable {
    value = local.some_variable_name
}
output testUnknownConflictingLocalVariable {
    value = local.example.another_test_attribute
}
