
#if EXPERIMENTAL

resource "blocks_resource" "a_resource" {
    dynamic "a_list_of_resources" {
        for_each = ["hi", "bye"]
        content {
            inner_string = a_list_of_resources.value
        }
    }
}

#endif