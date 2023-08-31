// Check we keep variable comments
config optStrIn string {
  // About the default
  default = "some string"
 // More type comments
}
    // About the bool local
    aBool = true
aDataSource = invoke("complex:index/index:data_source", {
    // About properties
    aBool = aBool,
    aNumber = 2.3, // Trailing comments on properties
    aString = optStrIn,
    aListOfInts = [1, 2, 3],
    aMapOfBool = {
        // In maps
        a: true
        b: false
    },
innerObject = {
        // In objects
        innerString = "hello again"
    }
})
// Check that we keep resource comments
resource aResource "complex:index/index:resource" {
    // About properties
    aBool = true
    aNumber = 2.3
 // Trailing comments on properties
    aString = "hello world"
    aListOfInts = [1, 2, 3]
    aMapOfBool = {
        // In maps
        a: true
        b: false
    }
innerObject = {
        // In objects
        innerString = "hello again"
    }
}
// Check that we keep output comments
output someOutput {
    // About the output value
    value = aResource.result
}
