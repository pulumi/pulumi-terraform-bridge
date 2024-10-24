# The following example shows how to generate a unique name for an Azure Resource Group.

resource "random_uuid" "test" {
}

resource "azurerm_resource_group" "test" {
  name     = "${random_uuid.test.result}-rg"
  location = "Central US"
}
