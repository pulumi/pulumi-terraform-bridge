variable "input" {
    type = string
}

module "dup" {
    source = "../outer_mod"
    this_many = var.input * 2
}

output "result" {
    value = module.dup.text
}