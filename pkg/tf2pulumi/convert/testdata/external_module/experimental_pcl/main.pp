component "cidrs" "./subnets" {
  baseCidrBlock = "10.0.0.0/8"
  networks = [{
    name    = "foo"
    newBits = 8
    }, {
    name    = "bar"
    newBits = 8
  }]


}
output "blocks" {
  value = cidrs.networkCidrBlocks
}
