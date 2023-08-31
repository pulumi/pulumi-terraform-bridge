aDataSource = invoke("blocks:index/index:data_source", {
  aListOfResources = [{
    innerString = "hi"
    }, {
    innerString = "bye"
  }]
})

resource "aResource" "blocks:index/index:resource" {
  aListOfResources = [{
    innerString = "hi"
    }, {
    innerString = "bye"
  }]
}
