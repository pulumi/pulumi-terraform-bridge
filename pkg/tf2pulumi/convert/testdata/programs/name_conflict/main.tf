variable "a_thing" {

}

locals {
    a_thing = true
}

resource "simple_resource" "a_thing" {
    input_one = "Hello ${var.a_thing}"
    input_two = local.a_thing
}

data "simple_data_source" "a_thing" {
    input_one = "Hello ${simple_resource.a_thing.result}"
    input_two = local.a_thing
}

output "a_thing" {
    value = data.simple_data_source.a_thing.result
}
