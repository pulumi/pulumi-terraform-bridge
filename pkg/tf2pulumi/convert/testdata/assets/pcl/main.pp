resource aResource "assets:index:resource" {
    source = fileAsset("./filepath")
}
aDataSource = invoke("assets:index:data_source", {
    source = fileAsset("./filepath")
})
