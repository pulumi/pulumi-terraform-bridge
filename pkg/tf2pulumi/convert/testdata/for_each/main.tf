resource "simple_resource" "a_resource_with_foreach_map" {
    for_each = {
        cruel: "world"
        good: "class"
    }
    input_one =  "Hello ${each.key} ${each.value}"
    input_two = 0
}

output "some_output_a" {
    value = simple_resource.a_resource_with_foreach_map["cruel"].result
}

data "simple_data_source" "a_data_source_with_foreach_map" {
    for_each = {
        cruel: "world"
        good: "class"
    }
    input_one =  "Hello ${each.key} ${each.value}"
    input_two = true
}

output "some_output_b" {
    value = data.simple_data_source.a_data_source_with_foreach_map["cruel"].result
}

resource "simple_resource" "a_resource_with_foreach_array" {
    for_each = ["cruel", "good"]
    input_one =  "Hello ${each.value} world"
    input_two = true
}

output "some_output_c" {
    value = simple_resource.a_resource_with_foreach_array["good"].result
}

data "simple_data_source" "a_data_source_with_foreach_array" {
    for_each = ["cruel", "good"]
    input_one =  "Hello ${each.value} world"
    input_two = true
}

output "some_output_d" {
    value = data.simple_data_source.a_data_source_with_foreach_array["good"].result
}