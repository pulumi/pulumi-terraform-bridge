component "aModuleWithForeachMap" "./modules/simple" {
  options {
    range = {
      cruel = "world"
      good  = "class"
    }
  }
  input = "Hello ${range.key} ${range.value}"
}

output "someOutputA" {
  value = aModuleWithForeachMap["cruel"].output
}

component "aModuleWithForeachArray" "./modules/simple" {
  options {
    range = ["cruel", "good"]
  }
  input = "Hello ${range.value} world"
}

output "someOutputB" {
  value = aModuleWithForeachArray["good"].output
}
