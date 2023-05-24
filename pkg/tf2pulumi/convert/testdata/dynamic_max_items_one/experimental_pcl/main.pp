resource "main" "maxItemsOne:index/index:resource" {
  innerResource = [for entry in entries([true]) : {
    someInput = true
  }][0]
}
