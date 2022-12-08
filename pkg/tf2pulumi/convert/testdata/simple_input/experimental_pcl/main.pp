config "optStrIn" "string" {
  default = "some string"
}
config "numberIn" "number" {
}
output "regionOut" {
  value = optStrIn
}
output "numberOut" {
  value = numberIn
}
