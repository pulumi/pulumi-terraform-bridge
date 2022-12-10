resource aResource "simple:index:resource" {
    inputOne = "hello"
    inputTwo = true
}
output someOutput {
    value = aResource.result
}