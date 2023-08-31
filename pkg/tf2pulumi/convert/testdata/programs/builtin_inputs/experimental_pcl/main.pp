component "someModule" "./mod" {
}

output "modulePath" {
  value = someModule.output
}

output "root" {
  value = notImplemented("path.root")
}

output "cwd" {
  value = notImplemented("path.cwd")
}

output "workspace" {
  value = notImplemented("terraform.workspace")
}
