resource "main" "maxItemsOne:index/index:resource" {
  aliases = [{
    ensureHealth = true
  }]
}

unknownDatasource = invoke("maxItemsOne:index/index:datasource", {
  aliases = {
    ensureHealth = true
  }
})
