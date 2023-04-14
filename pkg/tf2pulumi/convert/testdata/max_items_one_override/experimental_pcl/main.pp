resource "main" "maxItemsOne:index/index:resource" {
  aliases = [{
    ensureHealth = true
  }]
}
someDatasource = invoke("maxItemsOne:index/index:datasource", {
  aliases = {
    ensureHealth = true
  }
})
