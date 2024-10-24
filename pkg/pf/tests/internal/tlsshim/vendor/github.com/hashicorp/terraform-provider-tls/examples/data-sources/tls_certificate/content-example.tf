data "tls_certificate" "example_content" {
  content = file("example.pem")
}