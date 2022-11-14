variable "opt_str_in" {
  default = "some string"
}

variable "number_in" {
    type = number
}

output "region_out" {
    value = var.opt_str_in
}

output "number_out" {
    value = var.number_in
}