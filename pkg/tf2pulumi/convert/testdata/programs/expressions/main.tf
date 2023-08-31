output "null_out" {
    value = null
}

output "number_out" {
    value = 0
}

output "bool_out" {
    value = true
}

output "string_out" {
    value = "hello world"
}

output "tuple_out" {
    value = [1, 2, 3]
}

output "number_operators_out" {
    value = -(1 + 2) * 3 / 4 % 5
}

output "bool_operators_out" {
    value = !(true || false) && true
}

output "str_object_out" {
    value = {
        hello: "hallo"
        goodbye: "ha det"
    }
}

locals {
    a_key = "hello"
    a_value = -1
    a_list = [1, 2, 3]
    a_list_of_maps = [
        {
            x: [1, 2]
            y: [3, 4]
        },
        {
            x: [5, 6]
            y: [7, 8]
        }
    ]
}

output "static_index_out" {
    value = local.a_list[1]
}

output "dynamic_index_out" {
    value = local.a_list[local.a_value]
}

output "complex_object_out" {
    value = {
        a_tuple: ["a", "b", "c"]
        an_object: {
            literal_key: 1
            another_literal_key = 2
            "yet_another_literal_key": local.a_value
            // This only translates correctly in the new converter.
            (local.a_key) = 4
        }
        ambiguous_for: {
            "for" = 1
        }
    }
}

output "simple_template" {
    value = "${local.a_value}"
}

output "quoted_template" {
    value = "The key is ${local.a_key}"
}

output "heredoc" {
    value = <<END
This is also a template.
So we can output the key again ${local.a_key}
END
}

output "for_tuple" {
    value = [for key, value in ["a", "b"] : "${key}:${value}:${local.a_value}" if key != 0]
}

output "for_tuple_value_only" {
    value = [for value in ["a", "b"] : "${value}:${local.a_value}"]
}

output "for_tuple_value_only_attr" {
    value = [for x in [{id="i-123",zone="us-west"},{id="i-abc",zone="us-east"}]: x.id if x.zone == "us-east"]
}

output "for_object" {
    value = {for key, value in ["a", "b"] : key => "${value}:${local.a_value}" if key != 0}
}

output "for_object_grouping" {
    value = {for key, value in ["a", "a", "b"] : key => value... if key > 0}
}

output "relative_traversal_attr" {
    value = local.a_list_of_maps[0].x
}

output "relative_traversal_index" {
    value = local.a_list_of_maps[0]["x"]
}

output "conditional_expr" {
    value = local.a_value == 0 ? "true" : "false"
}