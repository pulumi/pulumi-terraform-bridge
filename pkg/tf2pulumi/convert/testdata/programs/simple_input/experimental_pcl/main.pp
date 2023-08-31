config "optStrIn" "string" {
  default = "some string"
}

config "numberIn" "number" {
}

config "anyIn" {
}

output "regionOut" {
  value = optStrIn
}

output "numberOut" {
  value = numberIn
}

output "anyOut" {
  value = anyIn
}
