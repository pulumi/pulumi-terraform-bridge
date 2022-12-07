resource "simple_resource" "a_resource" {
    input_one = "hello"
    input_two = true
}

output "some_output" {
    value = simple_resource.a_resource.result
}