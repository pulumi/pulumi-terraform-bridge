resource "complex_resource" "a_resource" {
    a_map_of_bool = {
        "kubernetes.io/role/elb" = true
        "tricky.name" = false
    }
}