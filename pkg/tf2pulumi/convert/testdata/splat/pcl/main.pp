resource aResourceWithCount "simple:index:resource" {
options {
    range = 4

}
    inputOne =  "Hello ${range.value}"
    inputTwo = true
}
output someOutputA {
    value = aResourceWithCount[*].result

}
resource aResourceWithForeachMap "simple:index:resource" {
options {
    range = {
        cruel: "world"
        good: "class"
    }

}
    inputOne =  "Hello ${range.key} ${range.value}"
    inputTwo = 0
}
output someOutputB {
    value = aResourceWithForeachMap[*].result

}