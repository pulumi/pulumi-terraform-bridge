config "region" "string" {
  description = "The region to use"
}

resource "aDefaultResource" "configured:index:resource" {
  __logicalName = "a_default_resource"
  inputOne      = region
}
