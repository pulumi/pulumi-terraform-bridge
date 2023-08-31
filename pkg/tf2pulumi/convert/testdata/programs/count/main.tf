resource "simple_resource" "a_resource_with_count" {
    count = 4
    input_one =  "Hello ${count.index}"
    input_two = true
}

output "some_output_a" {
    value = simple_resource.a_resource_with_count[0].result
}

output "some_output_b" {
    value = simple_resource.a_resource_with_count[1].result
}

data "simple_data_source" "a_data_source_with_count" {
    count = 2
    input_one = "Hello ${count.index}"
    input_two = true
}

output "some_output_c" {
    value = data.simple_data_source.a_data_source_with_count[0].result
}

output "some_output_d" {
    value = data.simple_data_source.a_data_source_with_count[1].result
}