aDataSource = invoke("simple:index:data_source", {
  inputOne = "hello"
  inputTwo = true
})

output "someOutput" {
  value = aDataSource.result
}
