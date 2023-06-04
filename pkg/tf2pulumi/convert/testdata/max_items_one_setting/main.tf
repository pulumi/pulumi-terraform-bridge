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

// Here, the target expression on the right hand side already marked with max items = 1
// so we will not have to project it into a singleton. The translation keeps this as is.
resource "maxItemsOne_resource" "resource_from_output_field" {
  innerResource = maxItemsOne_resource.resource_list.innerResourceOutput
}

// Indexing the field innerResourceOutput at zero should just remove the index
// since this field is marked with max items = 1
resource "maxItemsOne_resource" "resource_from_output_field_indexed" {
  innerResource = maxItemsOne_resource.resource_list.innerResourceOutput[0]
}


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