data "simpledata_source" "a_data_source" {
    input_one = "hello"
    input_two = true
}

output "some_output" {
    value = data.simpledata_source.a_data_source.result
}