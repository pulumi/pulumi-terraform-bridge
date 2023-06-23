component "someModule" "./mod" {
  input = "goodbye"
}

component "dup" "./outer_mod" {
  thisMany = 2
}

output "someOutput" {
  value = someModule.result + dup.text
}
