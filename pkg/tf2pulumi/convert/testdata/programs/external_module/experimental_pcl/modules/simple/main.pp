config "input" "string" {
}

resource "aResource" "simple:index:resource" {
  __logicalName = "a_resource"
  inputOne      = input
  inputTwo      = true
}

output "output" {
  value = aResource.result
}
