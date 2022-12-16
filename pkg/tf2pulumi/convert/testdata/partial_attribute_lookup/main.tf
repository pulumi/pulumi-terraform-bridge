resource "simple_resource" "example" {
    input_one = data.some_data_source.example.attr
    input_two = some_resource_type.example.list[0]
}