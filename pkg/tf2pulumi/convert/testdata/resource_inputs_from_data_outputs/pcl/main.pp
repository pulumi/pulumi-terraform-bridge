exampledata_source = invoke("simple:index:data_source", {
    inputOne = "hello",
    inputTwo = true
})
resource exampleresource "simple:index:resource" {
    inputOne = exampledata_source.inputOne
    inputTwo = exampledata_source.inputTwo
}
output example {
    value = exampleresource.result
}
