variable "input" {
    type = string
}

resource "simple_resource" "a_resource" {
    input_one = var.input
    input_two = true
}

output "output" {
    value = simple_resource.a_resource.result
}