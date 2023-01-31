    # A load of the examples in the docs use `path.module` which _should_ resolve to the file system path of
    # the current module, but tf2pulumi doesn't support that so we replace it with local.path_module.
    pathModule = "some/path"
    # Some of the examples in the docs use `path.root` which _should_ resolve to the file system path of the
    # root module of the configuration, but tf2pulumi doesn't support that so we replace it with
    pathRoot = "root/path"
# Examples for element
output funcElement0 {
  value = element(["a", "b", "c"], 1)
}
output funcElement1 {
  value = element(["a", "b", "c"], 3)
}
output funcElement2 {
  value = element(["a", "b", "c"], length(["a", "b", "c"])-1)
}
# Examples for file
output funcFile {
  value = readFile("${pathModule}/hello.txt")
}
# Examples for filebase64
output funcFilebase64 {
  value = filebase64("${pathModule}/hello.txt")
}
# Examples for filebase64sha256
output funcFilebase64sha256 {
  value = filebase64sha256("hello.txt")
}
# Examples for jsonencode
output funcJsonencode {
  value = toJSON({"hello"="world"})
}
# Examples for length
output funcLength0 {
  value = length([])
}
output funcLength1 {
  value = length(["a", "b"])
}
output funcLength2 {
  value = length({"a" = "b"})
}
output funcLength3 {
  value = length("hello")
}
output funcLength4 {
  value = length("👾🕹️")
}
# Examples for lookup
output funcLookup0 {
  value = lookup({a="ay", b="bee"}, "a", "what?")
}
output funcLookup1 {
  value = lookup({a="ay", b="bee"}, "c", "what?")
}
# Examples for sha1
output funcSha1 {
  value = sha1("hello world")
}
# Examples for split
output funcSplit0 {
  value = split(",", "foo,bar,baz")
}
output funcSplit1 {
  value = split(",", "foo")
}
output funcSplit2 {
  value = split(",", "")
}
