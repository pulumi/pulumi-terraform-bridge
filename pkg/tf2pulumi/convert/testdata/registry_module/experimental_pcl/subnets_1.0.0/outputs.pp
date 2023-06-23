output "networkCidrBlocks" {
  value = notImplemented("tomap(local.addrs_by_name)")
}

output "networks" {
  value = notImplemented("tolist(local.network_objs)")
}

output "baseCidrBlock" {
  value = baseCidrBlock
}
