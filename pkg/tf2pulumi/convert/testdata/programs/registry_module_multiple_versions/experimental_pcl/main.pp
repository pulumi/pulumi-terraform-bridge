component "dir-100" "./dir_1.0.0" {
  baseDir = "./src"
  templateVars = {
    vpcId = "vpc-abc123"
  }
}

component "dir-102" "./dir_1.0.2" {
  baseDir = "./otherSrc"
  templateVars = {
    someFlag = true
  }
}
