data "blocks_data_source" "a_data_source" {
    a_list_of_resources {
        inner_string = "hi"
    }
    a_list_of_resources {
        inner_string = "bye"
    }
}

resource "blocks_resource" "a_resource" {
    a_list_of_resources {
        inner_string = "hi"
    }
    a_list_of_resources {
        inner_string = "bye"
    }
}