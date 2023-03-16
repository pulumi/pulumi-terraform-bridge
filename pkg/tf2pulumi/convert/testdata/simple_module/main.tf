#if EXPERIMENTAL

module "some_module" {
    source = "./mod"

    input = "goodbye"
}

output "some_output" {
    value = module.some_module.output
}

#else
// Modules are not supported
#endif