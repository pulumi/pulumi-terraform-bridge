resource "example" "simple:index:resource" {
  inputOne = exampledataunknownDataSource.attr
  inputTwo = exampleunknownResourceType.list[0]
}
resource "anotherExample" "simple:index:resource" {
  inputOne = resourceName.someAttribute
  inputTwo = exampleConfig.testAttribute
}
output "testUnknownAlreadyDeclaredDataSource" {
  value = anotherTestAttribute
}
output "testUnknownLocalVariable" {
  value = someVariableName
}
output "testUnknownAlreadyDeclaredLocalVariable" {
  value = theirexample.anotherTestAttribute
}
