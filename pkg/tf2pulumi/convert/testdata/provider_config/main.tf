# Test for provider blocks feature in Terraform https://developer.hashicorp.com/terraform/language/providers/configuration

provider "configured" {
    string_config = "a string"
    list_config = ["a", "list"]
    renamed_config = "a different pulumi name"
    object_config {
        inner_string = "an object"
    }
}

resource "configured_resource" "a_default_resource" {
    input_one = "hi"
}