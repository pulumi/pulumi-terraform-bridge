resource "tls_locally_signed_cert" "example" {
  cert_request_pem   = file("cert_request.pem")
  ca_private_key_pem = file("ca_private_key.pem")
  ca_cert_pem        = file("ca_cert.pem")

  validity_period_hours = 12

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}
