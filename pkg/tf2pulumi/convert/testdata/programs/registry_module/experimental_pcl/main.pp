component "cidrs" "./subnets_1.0.0" {
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
