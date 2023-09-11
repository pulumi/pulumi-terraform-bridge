resource "simple_resource" "example" {
    input_one = each.key
    input_two = each.value
}