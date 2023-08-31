resource "aResource" "complex:index/index:resource" {
  __logicalName = "a_resource"
  aBool         = true
  aNumber       = 2.3
  aString       = "hello world"
  aListOfInts   = [1, 2, 3]
  aMapOfBool = {
    a = true
    b = false
  }
  innerObject = {
    innerString = "hello again"
  }
}

output "someOutput" {
  value = aResource.result
}
