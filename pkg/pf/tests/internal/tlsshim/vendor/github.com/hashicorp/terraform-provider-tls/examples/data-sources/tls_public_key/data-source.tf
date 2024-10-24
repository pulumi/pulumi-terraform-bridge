resource "tls_private_key" "ed25519-example" {
  algorithm = "ED25519"
}

# Public key loaded from a terraform-generated private key, using the PEM (RFC 1421) format
data "tls_public_key" "private_key_pem-example" {
  private_key_pem = tls_private_key.ed25519-example.private_key_pem
}

# Public key loaded from filesystem, using the Open SSH (RFC 4716) format
data "tls_public_key" "private_key_openssh-example" {
  private_key_openssh = file("~/.ssh/id_rsa_rfc4716")
}
