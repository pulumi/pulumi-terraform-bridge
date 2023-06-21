config "howMany" "number" {
}

component "dup" "../../outer_mod" {
  thisMany = howMany * 2
}

output "result" {
  value = dup.text
}
