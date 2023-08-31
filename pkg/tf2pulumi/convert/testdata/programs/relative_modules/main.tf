#if EXPERIMENTAL

module "some_module" {
    source = "./mod"

    input = "goodbye"
}

module "dup" {
    source = "./outer_mod"
    this_many = 2
}

output "some_output" {
    value = module.some_module.result + module.dup.text
}

#else
// Modules are not supported
#endif