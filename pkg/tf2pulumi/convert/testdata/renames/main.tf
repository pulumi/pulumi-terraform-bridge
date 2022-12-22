resource "renames_resource" "a_resource" {
    a_number = 1
    a_resource = {
        inner_string = "hello"
    }
}

output "some_output_a" {
    value = renames_resource.a_resource.result
}

// This doesn't convert correctly
// Error: unknown property 'theInnerString' among [innerString]
// Error: cannot traverse value of type union(none, object({innerString = union(none, string)}, annotated(0xc000681c40)))
//output "some_output_b" {
//    value = data.renames_data_source.a_data_source.a_resource.inner_string
//}

data "renames_data_source" "a_data_source" {
    a_number = 2
    a_resource = {
        inner_string = "hello"
    }
}

output "some_output_c" {
    value = data.renames_data_source.a_data_source.result
}

// This doesn't convert correctly
//output "some_output_d" {
//    value = data.renames_data_source.a_data_source.a_resource.inner_string
//}