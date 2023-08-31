
resource "aResource" "blocks:index/index:resource" {
  __logicalName = "a_resource"
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}

resource "bResource" "blocks:index/index:resource" {
  __logicalName = "b_resource"
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}

resource "cResource" "blocks:index/index:resource" {
  __logicalName = "c_resource"
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}
