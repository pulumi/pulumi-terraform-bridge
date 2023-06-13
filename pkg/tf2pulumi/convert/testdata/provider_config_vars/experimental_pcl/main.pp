config "region" "string" {
  description = "The region to use"
}
resource "aDefaultResource" "configured:index:resource" {
  inputOne = region
}
