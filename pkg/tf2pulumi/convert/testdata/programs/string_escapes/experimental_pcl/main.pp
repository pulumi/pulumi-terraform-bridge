output "plainString" {
  value = "hello world"
}

output "escapedString" {
  value = "\"\thello\nworld\r1\\2\""
}

output "unicodeEscapeString" {
  value = "ᄑ"
}

output "unicodeString" {
  value = "Ǝ"
}
