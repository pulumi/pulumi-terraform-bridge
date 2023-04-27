// Check that lists as attributes are handled correctly
resource "renames_resource" "list_attr_resource" {
    a_list = [{inner_number=1}, {inner_number=2}]
}

// Check that lists as blocks are handled correctly
resource "renames_resource" "list_block_resource" {
    a_list {
        inner_number=1
    }
    a_list {
        inner_number=2
    }
}

#if EXPERIMENTAL
// Check that lists as dynamics are handled correctly
resource "renames_resource" "list_dynamic_resource" {
    dynamic "a_list" {
        for_each = [ 1, 2 ]
        content {
            inner_number = a_list.value
        }
    }
}
#endif