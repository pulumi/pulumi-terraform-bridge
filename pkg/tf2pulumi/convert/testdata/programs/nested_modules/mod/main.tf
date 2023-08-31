variable "input" {
    type = string
}

module "counter" {
    source = "./inner_mod"
    how_many = length(var.input)
}

output "output" {
    value = module.counter.result
}