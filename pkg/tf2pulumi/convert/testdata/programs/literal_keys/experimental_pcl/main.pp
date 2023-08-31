resource "aResource" "complex:index/index:resource" {
  __logicalName = "a_resource"
  aMapOfBool = {
    "kubernetes.io/role/elb" = true
    "tricky.name"            = false
  }
}
