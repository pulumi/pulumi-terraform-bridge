resource aResource "simple:index:resource" {
    input_one = "hello"
    input_two = true
}
output someOutput {
    value = aResource.result
}