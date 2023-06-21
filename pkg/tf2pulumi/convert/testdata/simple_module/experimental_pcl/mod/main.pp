config "input" "string" {
}

resource "aResource" "simple:index:resource" {
  inputOne = input
  inputTwo = true
}

output "output" {
  value = aResource.result
}
