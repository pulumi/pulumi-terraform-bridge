
resource "aResource" "blocks:index/index:resource" {
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}

resource "bResource" "blocks:index/index:resource" {
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}

resource "cResource" "blocks:index/index:resource" {
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}
