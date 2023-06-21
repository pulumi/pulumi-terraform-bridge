example = invoke("simple:index:data_source", {
  inputOne = "hello"
  inputTwo = true
})

resource "exampleResource" "simple:index:resource" {
  inputOne = example.inputOne
  inputTwo = example.inputTwo
}

output "example" {
  value = exampleResource.result
}
