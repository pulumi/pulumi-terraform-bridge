variable "number_in" {
    type = number
}

variable "string_in" {
    type = string
}

variable "bool_in" {
    type = bool
}

variable "string_list_in" {
    type = list(string)
}

variable "string_map_in" {
    type = map(string)
}

variable "object_in" {
    type = object({
       first = number,
       second = string
    })
}