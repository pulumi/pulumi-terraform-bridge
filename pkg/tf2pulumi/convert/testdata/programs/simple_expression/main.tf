variable "number_in" {
    type = number
}

output "expression_out" {
    value = "Hello ${var.number_in}"
}