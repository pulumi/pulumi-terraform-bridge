# Test for provisioners feature in Terraform https://developer.hashicorp.com/terraform/language/resources/provisioners/syntax
# In combination with for_each


config "echoData" "map(string)" {
  default = {
    first  = "First"
    second = "Second"
  }
}
resource "localExecResource" "simple:index:resource" {
  options {
    range = echoData
  }
  inputOne = "hello"
  inputTwo = true
}
resource "localExecResourceProvisioner0" "command:local:Command" {
  options {
    range     = echoData
    dependsOn = localExecResource
  }
  create = "echo ${range.value}"
}
