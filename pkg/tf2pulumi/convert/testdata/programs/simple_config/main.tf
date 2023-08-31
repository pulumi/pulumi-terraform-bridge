variable "number_in" {
    type = number
    description = "This is an example of a variable description"
}

variable "string_in" {
    type = string
}

variable "nullable_string_in" {
    type = string
    default = null
}

variable "opt_any_in" {
  default = null
}

variable "any_with_default" {
  type = any
  default = {}
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

#if EXPERIMENTAL
variable "string_map_any_in" {
    type = map(any)
}
#endif

variable "object_in" {
    type = object({
       first = number,
       second = string
    })
}