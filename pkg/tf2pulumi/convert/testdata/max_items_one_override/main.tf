resource "maxItemsOne_resource" "main" {
    aliases {
        ensureHealth = true
    }
}

data "maxItemsOne_datasource" "unknown_datasource" {
  aliases {
    ensureHealth = true
  }
}