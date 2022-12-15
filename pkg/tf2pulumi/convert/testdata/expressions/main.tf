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
}

output "index_out" {
    value = local.a_list[1]
}

output "complex_object_out" {
    value = {
        a_tuple: ["a", "b", "c"]
        an_object: {
            literal_key: 1
            another_literal_key = 2
            "yet_another_literal_key": local.a_value
            # This doesn't translate correctly
            # (local.a_key) = 4
        }
        ambiguous_for: {
            "for" = 1
        }
    }
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

output "for_object" {
    value = {for key, value in ["a", "b"] : key => "${value}:${local.a_value}" if key != 0}
}