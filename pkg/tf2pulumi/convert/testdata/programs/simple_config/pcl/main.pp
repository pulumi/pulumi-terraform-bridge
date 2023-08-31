config numberIn number {
}
config stringIn string {
}
config nullableStringIn string {
    default = null
}
config optAnyIn {
  default = null
}
config anyWithDefault {
  default = {}
}
config boolIn bool {
}
config stringListIn "list(string)" {
}
config stringMapIn "map(string)" {
}
config objectIn "object({first = number, second = string})" {
}
