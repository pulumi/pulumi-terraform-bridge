variable "opt_str_in" {
  default = "some string"
}

variable "number_in" {
    type = number
}

variable "any_in" {
}

output "region_out" {
    value = var.opt_str_in
}

output "number_out" {
    value = var.number_in
}

output "any_out" {
    value = var.any_in
}
