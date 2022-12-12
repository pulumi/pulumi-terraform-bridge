config aThingInput {
}
    myAThing = true
resource aThingresource "simple:index:resource" {
    inputOne = "Hello ${aThingInput}"
    inputTwo = myAThing
}
aThingdata_source = invoke("simple:index:data_source", {
    inputOne = "Hello ${aThingresource.result}",
    inputTwo = myAThing
})
output aThing {
    value = aThingdata_source.result
}
