
#if EXPERIMENTAL
resource "maxItemsOne_resource" "main" {
    dynamic "innerResource" {
        for_each = [true]
        content {
            someInput = true
        }
    }
}
#endif