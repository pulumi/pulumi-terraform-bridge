
#if EXPERIMENTAL

resource "blocks_resource" "a_resource" {
    dynamic "a_list_of_resources" {
        for_each = ["hi", "bye"]
        content {
            inner_string = a_list_of_resources.value
        }
    }
}

resource "blocks_resource" "b_resource" {
    dynamic "a_list_of_resources" {
        for_each = ["hi", "bye"]
        iterator = "thing"

        content {
            inner_string = thing.value
        }
    }
}

resource "blocks_resource" "c_resource" {
    dynamic "a_list_of_resources" {
        for_each = ["hi", "bye"]
        iterator = "each"

        content {
            inner_string = each.value
        }
    }
}

#endif