resource example "simple:index:resource" {
    inputOne = data.unknown_data_source.example.attr
    inputTwo = unknown_resource_type.example.list[0]
}
