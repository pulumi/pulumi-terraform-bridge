resource "simple_resource" "a_resource" {
    timeouts {
        create = "60m"
        delete = "2h"
    }

    input_one = "hello"
    input_two = true
}