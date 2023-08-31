// Check that lists as attributes are handled correctly
resource "listAttrResource" "renames:index/index:resource" {
  __logicalName = "list_attr_resource"
  theList = [{
    number = 1
    }, {
    number = 2
  }]
}


// Check that lists as blocks are handled correctly
resource "listBlockResource" "renames:index/index:resource" {
  __logicalName = "list_block_resource"
  theList = [{
    number = 1
    }, {
    number = 2
  }]
}


// Check that lists as dynamics are handled correctly
resource "listDynamicResource" "renames:index/index:resource" {
  __logicalName = "list_dynamic_resource"
  theList = [for entry in entries([1, 2]) : {
    number = entry.value
  }]
}
