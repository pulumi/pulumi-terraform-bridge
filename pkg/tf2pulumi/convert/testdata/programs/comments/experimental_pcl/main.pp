// Check we keep variable comments
config "optStrIn" "string" {
  default = "some string"
}

// About the bool local
aBool = true
// Trailing bool comment



// Check we keep data source comments
aDataSource = invoke("complex:index/index:data_source", {

  // About properties
  aBool   = aBool
  aNumber = 2.3 // Trailing comments on properties

  // Trailing comments on properties
  aString     = optStrIn
  aListOfInts = [1, 2, 3]
  aMapOfBool = {
    a = true
    b = false
  }
  innerObject = {

    // In objects
    innerString = "hello again"
  }
})


// Check that we keep resource comments
resource "aResource" "complex:index/index:resource" {
  __logicalName = "a_resource"
  aBool         = true
  aNumber       = 2.3 // Trailing comments on properties

  aString     = "hello world"
  aListOfInts = [1, 2, 3]
  aMapOfBool = {
    a = true
    b = false
  }
  innerObject = {

    // In objects
    innerString = "hello again"
  }
}


// Check that we keep output comments
output "someOutput" {

  // About the output value
  value = aResource.result
}
