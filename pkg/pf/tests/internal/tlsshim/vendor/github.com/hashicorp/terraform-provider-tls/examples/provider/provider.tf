# This example creates a self-signed certificate,
# and uses it to create an AWS IAM Server certificate.
#
# THIS IS NOT RECOMMENDED FOR PRODUCTION SERVICES.
# See the detailed documentation of each resource for further
# security considerations and other practical tradeoffs.

resource "tls_private_key" "example" {
  algorithm = "ECDSA"
}

resource "tls_self_signed_cert" "example" {
  key_algorithm   = tls_private_key.example.algorithm
  private_key_pem = tls_private_key.example.private_key_pem

  # Certificate expires after 12 hours.
  validity_period_hours = 12

  # Generate a new certificate if Terraform is run within three
  # hours of the certificate's expiration time.
  early_renewal_hours = 3

  # Reasonable set of uses for a server SSL certificate.
  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]

  dns_names = ["example.com", "example.net"]

  subject {
    common_name  = "example.com"
    organization = "ACME Examples, Inc"
  }
}

# For example, this can be used to populate an AWS IAM server certificate.
resource "aws_iam_server_certificate" "example" {
  name             = "example_self_signed_cert"
  certificate_body = tls_self_signed_cert.example.cert_pem
  private_key      = tls_private_key.example.private_key_pem
}
