resource "random_bytes" "jwt_secret" {
  length = 64
}

resource "azurerm_key_vault_secret" "jwt_secret" {
  key_vault_id = "some-azure-key-vault-id"
  name         = "JwtSecret"
  value        = random_bytes.jwt_secret.base64
}
