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



// Here, the target expression on the right hand side already marked with max items = 1
// so we will not have to project it into a singleton. The translation keeps this as is.
resource "resourceFromOutputField" "maxItemsOne:index/index:resource" {
  innerResource = resourceList.innerResourceOutput
}


// Indexing the field innerResourceOutput at zero should just remove the index
// since this field is marked with max items = 1
resource "resourceFromOutputFieldIndexed" "maxItemsOne:index/index:resource" {
  innerResource = resourceList.innerResourceOutput
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
