variable "this_many" {
    type = number
}

output "text" {
    value = var.this_many == 1 ? "one" : "many"
}