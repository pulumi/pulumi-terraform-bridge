#if EXPERIMENTAL

variable "simple_object_config" {
    type = object({
        first_member = number,
        second_member = string
    })

    default = {
        first_member = 10
        second_member = "hello"
    }
}

variable "object_list_config" {
    type = list(object({
        first_member = number,
        second_member = string
    }))

    default = [{
        first_member = 10
        second_member = "hello"
    }]
}

variable "object_list_config_empty" {
    type = list(object({
        first_member = number,
        second_member = string
    }))

    default = []
}

variable "object_map_config" {
    type = map(object({
        first_member = number,
        second_member = string
    }))

    default = {
        "hello" = {
             first_member = 10
             second_member = "hello"
        }
    }
}

variable "object_map_config_empty" {
    type = map(object({
        first_member = number,
        second_member = string
    }))

    default = {}
}

resource "simple_resource" "using_simple_object_config" {
    input_one = var.simple_object_config.first_member
}

resource "simple_resource" "using_list_object_config" {
    input_one = var.object_list_config[0].first_member
}

resource "simple_resource" "using_list_object_config_for_each" {
    for_each = var.object_list_config
    input_one = each.value.first_member
}

resource "simple_resource" "using_map_object_config" {
    input_one = var.object_map_config["hello"].first_member
}

resource "simple_resource" "using_map_object_config_for_each" {
    for_each = var.object_map_config
    input_one = each.value.first_member
}

resource "blocks_resource" "using_dynamic" {
    dynamic "a_list_of_resources" {
        for_each = var.object_map_config
        content {
            inner_string = a_list_of_resources.value.first_member
        }
    }
}

resource "blocks_resource" "using_dynamic_iterator" {
    dynamic "a_list_of_resources" {
        for_each = var.object_map_config
        iterator = "each"
        content {
            inner_string = each.value.first_member
        }
    }
}

#endif