resource "main" "maxItemsOne:index/index:resource" {
  innerResource = singleOrNone([for entry in entries([true]) : {
    someInput = true
  }])
}
