resource "simple_resource" "a_resource_with_count" {
    count = 4
    input_one =  "Hello ${count.index}"
    input_two = true
}

output "some_output_a" {
    value = simple_resource.a_resource_with_count[*].result
}

resource "simple_resource" "a_resource_with_foreach_map" {
    for_each = {
        cruel: "world"
        good: "class"
    }
    input_one =  "Hello ${each.key} ${each.value}"
    input_two = 0
}

output "some_output_b" {
    value = simple_resource.a_resource_with_foreach_map[*].result
}