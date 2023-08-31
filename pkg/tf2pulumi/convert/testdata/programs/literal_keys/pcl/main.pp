resource aResource "complex:index/index:resource" {
    aMapOfBool = {
        "kubernetes.io/role/elb" = true
        "tricky.name" = false
    }
}
