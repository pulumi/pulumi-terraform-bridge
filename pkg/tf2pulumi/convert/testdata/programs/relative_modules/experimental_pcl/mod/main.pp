config "input" "string" {
}

component "dup" "../outer_mod" {
  thisMany = input * 2
}

output "result" {
  value = dup.text
}
