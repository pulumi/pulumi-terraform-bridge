# Test for provisioners feature in Terraform https://developer.hashicorp.com/terraform/language/resources/provisioners/syntax
resource "localExecResource" "simple:index:resource" {
  __logicalName = "local_exec_resource"
  inputOne      = "hello"
  inputTwo      = true

}
resource "localExecResourceProvisioner0" "command:local:Command" {
  options {
    dependsOn = [localExecResource]
  }
  create = "echo first"
}
resource "localExecResourceProvisioner1" "command:local:Command" {
  options {
    dependsOn = [localExecResourceProvisioner0]
  }
  create = "true"
  update = "true"
  delete = "echo second"
}
resource "localExecResourceProvisioner2" "command:local:Command" {
  options {
    dependsOn = [localExecResourceProvisioner1]
  }
  create      = "echo third"
  interpreter = ["/bin/bash", "-c"]
}
resource "localExecResourceProvisioner3" "command:local:Command" {
  options {
    dependsOn = [localExecResourceProvisioner2]
  }
  create = "echo forth"
  environment = {
    FOO = "bar"
  }
}
