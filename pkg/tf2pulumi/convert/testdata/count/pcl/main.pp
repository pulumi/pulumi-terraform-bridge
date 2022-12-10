resource aResourceWithCount "simple:index:resource" {
options {
    range = 4

}
    inputOne =  "Hello ${range.value}"
    inputTwo = true
}
output someOutputA {
    value = aResourceWithCount[0].result
}
output someOutputB {
    value = aResourceWithCount[1].result
}