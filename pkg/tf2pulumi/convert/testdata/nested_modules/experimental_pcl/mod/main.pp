config "input" "string" {
}

component "counter" "./inner_mod" {
  howMany = length(input)
}

output "output" {
  value = counter.result
}
