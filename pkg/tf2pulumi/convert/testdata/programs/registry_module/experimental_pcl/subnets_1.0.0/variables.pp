config "baseCidrBlock" "string" {
  description = "A network address prefix in CIDR notation that all of the requested subnetwork prefixes will be allocated within."
}

config "networks" "list(object({name=string, newBits=number}))" {
  description = "A list of objects describing requested subnetwork prefixes. new_bits is the number of additional network prefix bits to add, in addition to the existing prefix on base_cidr_block."
}
