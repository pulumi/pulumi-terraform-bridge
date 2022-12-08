resource aResourceWithCount "simple:index:resource" {
options {
    range = 4

}
    input_one =  "Hello ${range.value}"
    input_two = true
}
output someOutputA {
    value = aResourceWithCount[0].result
}
output someOutputB {
    value = aResourceWithCount[1].result
}