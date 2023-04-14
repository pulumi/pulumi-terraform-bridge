resource "maxItemsOne_resource" "main" {
    aliases {
        ensureHealth = true
    }
}

data "maxItemsOne_datasource" "some_datasource" {
  aliases {
    ensureHealth = true
  }
}