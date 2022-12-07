data "simple_data_source" "a_data_source" {
    input_one = "hello"
    input_two = true
}

output "some_output" {
    value = data.simple_data_source.a_data_source.result
}