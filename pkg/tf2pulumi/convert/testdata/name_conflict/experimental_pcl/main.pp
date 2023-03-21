config "aThing" {
}
myaThing = true
resource "aThingResource" "simple:index:resource" {
  inputOne = "Hello ${aThing}"
  inputTwo = myaThing
}
aThingData = invoke("simple:index:data_source", {
  inputOne = "Hello ${aThingResource.result}"
  inputTwo = myaThing
})
output "aThingOutput" {
  value = aThingData.result
}
