resource "aResource" "blocks:index/index:resource" {
  aListOfResources = [for entry in entries(["hi", "bye"]) : {
    innerString = entry.value
  }]
}