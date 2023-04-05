variable "how_many" {
    type = number
}

module "dup" {
    source = "../../outer_mod"
    this_many = var.how_many * 2
}

output "result" {
    value = module.dup.text
}