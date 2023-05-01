config "simpleObjectConfig" "object({firstMember=number, secondMember=string})" {
  default = {
    firstMember  = 10
    secondMember = "hello"
  }
}
config "objectListConfig" "list(object({firstMember=number, secondMember=string}))" {
  default = [{
    firstMember  = 10
    secondMember = "hello"
  }]
}
config "objectListConfigEmpty" "list(object({firstMember=number, secondMember=string}))" {
  default = []
}
config "objectMapConfig" "map(object({firstMember=number, secondMember=string}))" {
  default = {
    hello = {
      firstMember  = 10
      secondMember = "hello"
    }
  }
}
config "objectMapConfigEmpty" "map(object({firstMember=number, secondMember=string}))" {
  default = {}
}
resource "usingSimpleObjectConfig" "simple:index:resource" {
  inputOne = simpleObjectConfig.firstMember
}
resource "usingListObjectConfig" "simple:index:resource" {
  inputOne = objectListConfig[0].firstMember
}
resource "usingListObjectConfigForEach" "simple:index:resource" {
  options {
    range = objectListConfig
  }
  inputOne = range.value.firstMember
}
resource "usingMapObjectConfig" "simple:index:resource" {
  inputOne = objectMapConfig["hello"].firstMember
}
resource "usingMapObjectConfigForEach" "simple:index:resource" {
  options {
    range = objectMapConfig
  }
  inputOne = range.value.firstMember
}
resource "usingDynamic" "blocks:index/index:resource" {
  aListOfResources = [for entry in entries(objectMapConfig) : {
    innerString = entry.value.firstMember
  }]
}
resource "usingDynamicIterator" "blocks:index/index:resource" {
  aListOfResources = [for entry in entries(objectMapConfig) : {
    innerString = entry.value.firstMember
  }]
}
