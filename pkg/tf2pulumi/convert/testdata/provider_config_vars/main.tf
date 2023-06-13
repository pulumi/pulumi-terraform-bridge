#if EXPERIMENTAL

variable "region" {
  type        = string
  description = "The region to use"
}

provider "configured" {
    string_config = var.region
}

resource "configured_resource" "a_default_resource" {
    input_one = var.region
}

#endif