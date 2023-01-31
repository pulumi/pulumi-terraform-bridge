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
output someOutputA {
    value = aResourceWithForeachMap["cruel"].result
}
aDataSourceWithForeachMap =[for __key, __value in {
        cruel: "world"
        good: "class"
    }
: invoke("simple:index:data_source", {
    inputOne =  "Hello ${__key} ${__value}",
    inputTwo = true
})]
output someOutputB {
    value = aDataSourceWithForeachMap["cruel"].result
}
resource aResourceWithForeachArray "simple:index:resource" {
options {
    range = ["cruel", "good"]

}
    inputOne =  "Hello ${range.value} world"
    inputTwo = true
}
output someOutputC {
    value = aResourceWithForeachArray["good"].result
}
aDataSourceWithForeachArray =[for __key, __value in ["cruel", "good"]
: invoke("simple:index:data_source", {
    inputOne =  "Hello ${__value} world",
    inputTwo = true
})]
output someOutputD {
    value = aDataSourceWithForeachArray["good"].result
}
