resource "aResourceWithCount" "simple:index:resource" {
  __logicalName = "a_resource_with_count"
  options {
    range = 4
  }
  inputOne = "Hello ${range.value}"
  inputTwo = true
}

output "someOutputA" {
  value = aResourceWithCount[*].result
}

resource "aResourceWithForeachMap" "simple:index:resource" {
  __logicalName = "a_resource_with_foreach_map"
  options {
    range = {
      cruel = "world"
      good  = "class"
    }
  }
  inputOne = "Hello ${range.key} ${range.value}"
  inputTwo = 0
}

output "someOutputB" {
  value = aResourceWithForeachMap[*].result
}
