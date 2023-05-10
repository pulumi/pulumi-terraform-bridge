variable "list_input" {
}

resource "maxItemsOne_resource" "resource_block" {
  innerResource {
      someInput = true
  }
}

resource "maxItemsOne_resource" "resource_list" {
  innerResource = [{someInput = true}]
}

#if EXPERIMENTAL
resource "maxItemsOne_resource" "resource_var" {
  innerResource = var.list_input
}
#endif

data "maxItemsOne_datasource" "datasource_block" {
  innerResource {
    someInput = true
  }
}

data "maxItemsOne_datasource" "datasource_list" {
  innerResource = [{someInput = true}]
}

#if EXPERIMENTAL
data "maxItemsOne_datasource" "datasource_var" {
  innerResource = var.list_input
}
#endif