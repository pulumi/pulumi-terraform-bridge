
#if EXPERIMENTAL

provider "configured" {
    dynamic "object_config" {
        for_each = ["a"]
        content {
            inner_string = object_config.value
        }
    }
}

#endif