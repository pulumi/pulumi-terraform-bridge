#if EXPERIMENTAL

module "a_module_with_foreach_map" {
    source = "../modules/simple"

    for_each = {
        cruel: "world"
        good: "class"
    }

    input = "Hello ${each.key} ${each.value}"
}

output "some_output_a" {
    value = module.a_module_with_foreach_map["cruel"].output
}

module "a_module_with_foreach_array" {
    source = "../modules/simple"

    for_each = ["cruel", "good"]

    input =  "Hello ${each.value} world"
}

output "some_output_b" {
    value = module.a_module_with_foreach_array["good"].output
}

#else
// Modules are not supported
#endif