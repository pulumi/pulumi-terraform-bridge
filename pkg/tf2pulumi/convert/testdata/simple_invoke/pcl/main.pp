aDataSource = invoke("simpledata::source", {
    input_one = "hello",
    input_two = true
})
output someOutput {
    value = aDataSource.result
}