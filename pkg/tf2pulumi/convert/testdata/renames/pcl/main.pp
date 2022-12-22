resource aResource "renames:index/index:resource" {
    theNumber = 1
    theResource = {
        theInnerString = "hello"
    }
}
output someOutputA {
    value = aResource.myResult
}// This doesn't convert correctly
// Error: unknown property 'theInnerString' among [innerString]
// Error: cannot traverse value of type union(none, object({innerString = union(none, string)}, annotated(0xc000681c40)))
//output "some_output_b" {
//    value = data.renames_data_source.a_data_source.a_resource.inner_string
//}

aDataSource = invoke("renames:index/index:data_source", {
    theNumber = 2,
    theResource = {
        theInnerString = "hello"
    }
})
output someOutputC {
    value = aDataSource.myResult
}
