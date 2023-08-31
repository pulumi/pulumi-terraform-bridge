#if EXPERIMENTAL

module "some_module" {
    source = "../modules/simple"

    input = "hello"
}

output "some_output" {
    value = module.some_module.output
}

#else
// Modules are not supported
#endif