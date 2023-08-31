config listInput {}
resource resourceBlock "maxItemsOne:index/index:resource" {
    innerResource = {
        someInput = true
    }
}
resource resourceList "maxItemsOne:index/index:resource" {
    innerResource = {
        someInput = true
    }
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
