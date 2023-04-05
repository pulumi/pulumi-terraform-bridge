output "networkCidrBlocks" {
  value = notImplemented("tomap(local.addrs_by_name)")
}
output "networksOutput" {
  value = notImplemented("tolist(local.network_objs)")
}
output "baseCidrBlockOutput" {
  value = baseCidrBlock
}
