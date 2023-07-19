resource "example" "simple:index:resource" {
  inputOne = exampledataunknownDataSource.attr
  inputTwo = exampleunknownResourceType.list[0]
}
resource "anotherExample" "simple:index:resource" {
  inputOne = resourceName.someAttribute
  inputTwo = someConfig.testAttribute
}
output "testUnknownAlreadyDeclaredDataSource" {
  value = exampledataunknownDataSource.anotherTestAttribute
}
output "testUnknownLocalVariable" {
  value = someVariableName
}
output "testUnknownConflictingLocalVariable" {
  value = theirexample.anotherTestAttribute
}
