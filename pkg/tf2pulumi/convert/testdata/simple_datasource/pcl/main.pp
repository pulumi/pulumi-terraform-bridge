aDataSource = invoke("simple:index:data_source", {
    input_one = "hello",
    input_two = true
})
output someOutput {
    value = aDataSource.result
}