component "someModule" "./modules/simple" {
  input = "hello"
}

output "someOutput" {
  value = someModule.output
}
