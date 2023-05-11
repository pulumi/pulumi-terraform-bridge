config "listInput" {
}
resource "resourceBlock" "maxItemsOne:index/index:resource" {
  innerResource = {
    someInput = true
  }
}
resource "resourceList" "maxItemsOne:index/index:resource" {
  innerResource = {
    someInput = true
  }
}
resource "resourceVar" "maxItemsOne:index/index:resource" {
  innerResource = listInput[0]
}
datasourceBlock = invoke("maxItemsOne:index/index:datasource", {
  innerResource = {
    someInput = true
  }
})
datasourceList = invoke("maxItemsOne:index/index:datasource", {
  innerResource = {
    someInput = true
  }
})
datasourceVar = invoke("maxItemsOne:index/index:datasource", {
  innerResource = listInput[0]
})
