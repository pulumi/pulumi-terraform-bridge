resource "aResourceWithCount" "simple:index:resource" {
  __logicalName = "a_resource_with_count"
  options {
    range = 4
  }
  inputOne = "Hello ${range.value}"
  inputTwo = true
}

output "someOutputA" {
  value = aResourceWithCount[0].result
}

output "someOutputB" {
  value = aResourceWithCount[1].result
}

aDataSourceWithCount = [for __index in range(2) : invoke("simple:index:data_source", {
  inputOne = "Hello ${__index}"
  inputTwo = true
})]

output "someOutputC" {
  value = aDataSourceWithCount[0].result
}

output "someOutputD" {
  value = aDataSourceWithCount[1].result
}
