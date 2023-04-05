#if EXPERIMENTAL

module "cidrs" {
  source = "hashicorp/subnets/cidr"
  version = "1.0.0"

  base_cidr_block = "10.0.0.0/8"
  networks = [
    {
      name     = "foo"
      new_bits = 8
    },
    {
      name     = "bar"
      new_bits = 8
    },
  ]
}

output "blocks" {
    value = module.cidrs.network_cidr_blocks
}

#else
// Modules are not supported
#endif