resource example "simple:index:resource" {
    inputOne = data.some_data_source.example.attr
    inputTwo = some_resource_type.example.list[0]
}
