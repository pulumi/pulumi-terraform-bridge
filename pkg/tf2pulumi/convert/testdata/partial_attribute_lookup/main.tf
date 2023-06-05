resource "simple_resource" "example" {
    input_one = data.unknown_data_source.example.attr
    input_two = unknown_resource_type.example.list[0]
}