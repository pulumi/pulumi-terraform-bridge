# Test for provisioners feature in Terraform https://developer.hashicorp.com/terraform/language/resources/provisioners/syntax
# In combination with for_each

#if EXPERIMENTAL

variable "echo_data" {
    type = map(string)
    default = {
        "first": "First"
        "second": "Second"
    }
}

resource "simple_resource" "local_exec_resource" {
    for_each = var.echo_data
    input_one = "hello"
    input_two = true

    provisioner "local-exec" {
        command = "echo ${each.value}"
    }
}
#endif