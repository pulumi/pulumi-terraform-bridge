#if EXPERIMENTAL

locals {
    // Check we can use terraform builtin functions here and that they are evaluated
    static_local = title("static")
}

provider "configured" {
    string_config = local.static_local
}

resource "configured_resource" "a_default_resource" {
    input_one = local.static_local
}

#endif