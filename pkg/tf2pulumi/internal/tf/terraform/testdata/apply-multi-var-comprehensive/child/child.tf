variable "num" {
}

variable "source_ids" {
  type = list(string)
}

variable "source_names" {
  type = list(string)
}

resource "test_thing" "multi_count_var" {
  count = var.num

  key = "child.multi_count_var.${count.index}"

  # Can pluck a single item out of a multi-var
  source_id = var.source_ids[count.index]
}

resource "test_thing" "whole_splat" {
  key = "child.whole_splat"

  # Can "splat" the ids directly into an attribute of type list.
  source_ids           = var.source_ids
  source_names         = var.source_names
  source_ids_wrapped   = ["${var.source_ids}"]
  source_names_wrapped = ["${var.source_names}"]
}
