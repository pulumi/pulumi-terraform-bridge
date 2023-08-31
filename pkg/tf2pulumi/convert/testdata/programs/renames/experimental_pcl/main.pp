resource "aResource" "renames:index/index:resource" {
  __logicalName = "a_resource"
  theNumber     = 1
  theResource = {
    theInnerString = "hello"
  }
}

output "someOutputA" {
  value = aResource.myResult
}


//output "some_output_b" {
//    value = data.renames_data_source.a_data_source.a_resource.inner_string
//}
// The above doesn't convert correctly
// Error: unknown property 'theInnerString' among [innerString]
// Error: cannot traverse value of type union(none, object({innerString = union(none, string)}, annotated(0xc000681c40)))
aDataSource = invoke("renames:index/index:data_source", {
  theNumber = 2
  theResource = {
    theInnerString = "hello"
  }
})

output "someOutputC" {
  value = aDataSource.myResult
}


// output "some_output_d" {
//     value = data.renames_data_source.a_data_source.a_resource.inner_string
// }
// The above doesn't convert correctly
// unknown property 'theInnerString' among [innerString];
resource "manyResource" "renames:index/index:resource" {
  __logicalName = "many_resource"
  options {
    range = 2
  }
  theNumber = 1
  theResource = {
    theInnerString = "hello"
  }
}
