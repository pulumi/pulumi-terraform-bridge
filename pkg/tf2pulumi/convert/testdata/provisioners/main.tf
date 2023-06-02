# Test for provisioners feature in Terraform https://developer.hashicorp.com/terraform/language/resources/provisioners/syntax

#if EXPERIMENTAL
resource "simple_resource" "local_exec_resource" {
    input_one = "hello"
    input_two = true

    provisioner "local-exec" {
        command = "echo first"
    }

    provisioner "local-exec" {
        command = "echo second"
        when = destroy
    }

    provisioner "local-exec" {
        command = "echo third"
        interpreter = ["/bin/bash", "-c"]
    }

    provisioner "local-exec" {
        command = "echo forth"
        environment = {
            FOO = "bar"
        }
    }
}
#endif