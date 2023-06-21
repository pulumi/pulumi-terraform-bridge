config "thisMany" "number" {
}

output "text" {
  value = thisMany == 1 ? "one" : "many"
}
