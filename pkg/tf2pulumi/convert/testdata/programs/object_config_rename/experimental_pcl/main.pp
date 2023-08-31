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
  __logicalName = "using_simple_object_config"
  inputOne      = simpleObjectConfig.firstMember
}

resource "usingListObjectConfig" "simple:index:resource" {
  __logicalName = "using_list_object_config"
  inputOne      = objectListConfig[0].firstMember
}

resource "usingListObjectConfigForEach" "simple:index:resource" {
  __logicalName = "using_list_object_config_for_each"
  options {
    range = objectListConfig
  }
  inputOne = range.value.firstMember
}

resource "usingMapObjectConfig" "simple:index:resource" {
  __logicalName = "using_map_object_config"
  inputOne      = objectMapConfig["hello"].firstMember
}

resource "usingMapObjectConfigForEach" "simple:index:resource" {
  __logicalName = "using_map_object_config_for_each"
  options {
    range = objectMapConfig
  }
  inputOne = range.value.firstMember
}

resource "usingDynamic" "blocks:index/index:resource" {
  __logicalName = "using_dynamic"
  aListOfResources = [for entry in entries(objectMapConfig) : {
    innerString = entry.value.firstMember
  }]
}

resource "usingDynamicIterator" "blocks:index/index:resource" {
  __logicalName = "using_dynamic_iterator"
  aListOfResources = [for entry in entries(objectMapConfig) : {
    innerString = entry.value.firstMember
  }]
}
