#if EXPERIMENTAL

module "some_module" {
    source = "./mod"
}

output "module_path" {
    value = module.some_module.output
}

output "root" {
    value = path.root
}

output "cwd" {
    value = path.cwd
}

output "workspace" {
    value = terraform.workspace
}

#endif