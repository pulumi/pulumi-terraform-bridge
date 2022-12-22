data "simple_data_source" "example" {
    input_one = "hello"
    input_two = true
}

resource "simple_resource" "example" {
    input_one = data.simple_data_source.example.input_one
    input_two = data.simple_data_source.example.input_two
}

output "example" {
    value = simple_resource.example.result
}