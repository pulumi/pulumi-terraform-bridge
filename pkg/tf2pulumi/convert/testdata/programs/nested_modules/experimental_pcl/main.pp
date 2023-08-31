component "someModule" "./mod" {
  input = "goodbye"
}

output "someOutput" {
  value = someModule.output
}
