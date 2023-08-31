resource "renames_resource" "a_resource" {
    a_number = 1
    a_resource {
        inner_string = "hello"
    }
}

output "some_output_a" {
    value = renames_resource.a_resource.result
}

//output "some_output_b" {
//    value = data.renames_data_source.a_data_source.a_resource.inner_string
//}
// The above doesn't convert correctly
// Error: unknown property 'theInnerString' among [innerString]
// Error: cannot traverse value of type union(none, object({innerString = union(none, string)}, annotated(0xc000681c40)))

data "renames_data_source" "a_data_source" {
    a_number = 2
    a_resource {
        inner_string = "hello"
    }
}

output "some_output_c" {
    value = data.renames_data_source.a_data_source.result
}

// output "some_output_d" {
//     value = data.renames_data_source.a_data_source.a_resource.inner_string
// }
// The above doesn't convert correctly
// unknown property 'theInnerString' among [innerString];

resource "renames_resource" "many_resource" {
    count = 2
    a_number = 1
    a_resource {
        inner_string = "hello"
    }
}

// output "the_inner_strings" {
//     value = renames_resource.many_resource[*].a_resource.inner_string
// }
// The above doesn't convert correctly
// unknown property 'aResource' among [urn myResult theNumber theResource id];