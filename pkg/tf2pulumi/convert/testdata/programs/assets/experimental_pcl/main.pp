resource "aResource" "assets:index:resource" {
  __logicalName = "a_resource"
  source        = fileAsset("./filepath")
}

aDataSource = invoke("assets:index:data_source", {
  source = fileAsset("./filepath")
})
