// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file implements a simple system for locally testing and efficiently
// debugging the HCL converter without the use of GitHub Actions.
package tfgen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HclConversion(t *testing.T) {

	//============================= HCL code to be given to the converter =============================//
	hcl := `
	data "azurerm_client_config" "current" {}

	resource "azurerm_resource_group" "example" {
	  name     = "example-resources"
	  location = "West Europe"
	}
	
	resource "azurerm_key_vault" "example" {
	  name                = "examplekv"
	  location            = azurerm_resource_group.example.location
	  resource_group_name = azurerm_resource_group.example.name
	  tenant_id           = data.azurerm_client_config.current.tenant_id
	  sku_name            = "standard"
	
	  purge_protection_enabled = true
	}
	
	resource "azurerm_key_vault_access_policy" "storage" {
	  key_vault_id = azurerm_key_vault.example.id
	  tenant_id    = data.azurerm_client_config.current.tenant_id
	  object_id    = azurerm_storage_account.example.identity.0.principal_id
	
	  key_permissions    = ["get", "create", "list", "restore", "recover", "unwrapkey", "wrapkey", "purge", "encrypt", "decrypt", "sign", "verify"]
	  secret_permissions = ["get"]
	}
	
	resource "azurerm_key_vault_access_policy" "client" {
	  key_vault_id = azurerm_key_vault.example.id
	  tenant_id    = data.azurerm_client_config.current.tenant_id
	  object_id    = data.azurerm_client_config.current.object_id
	
	  key_permissions    = ["get", "create", "delete", "list", "restore", "recover", "unwrapkey", "wrapkey", "purge", "encrypt", "decrypt", "sign", "verify"]
	  secret_permissions = ["get"]
	}
	
	
	resource "azurerm_key_vault_key" "example" {
	  name         = "tfex-key"
	  key_vault_id = azurerm_key_vault.example.id
	  key_type     = "RSA"
	  key_size     = 2048
	  key_opts     = ["decrypt", "encrypt", "sign", "unwrapKey", "verify", "wrapKey"]
	
	  depends_on = [
		azurerm_key_vault_access_policy.client,
		azurerm_key_vault_access_policy.storage,
	  ]
	}
	
	
	resource "azurerm_storage_account" "example" {
	  name                     = "examplestor"
	  resource_group_name      = azurerm_resource_group.example.name
	  location                 = azurerm_resource_group.example.location
	  account_tier             = "Standard"
	  account_replication_type = "GRS"
	
	  identity {
		type = "SystemAssigned"
	  }
	}
	
	resource "azurerm_storage_account_customer_managed_key" "example" {
	  storage_account_id = azurerm_storage_account.example.id
	  key_vault_id       = azurerm_key_vault.example.id
	  key_name           = azurerm_key_vault_key.example.name
	}
	`
	//=================================================================================================//

	// [go, nodejs, python, dotnet, schema]
	languageName := "go"

	// Creating the Code Generator which will translate our HCL program
	g, err := NewGenerator(GeneratorOptions{
		Version:      "version",
		Language:     Language(languageName),
		Debug:        false,
		SkipDocs:     false,
		SkipExamples: false,
	})
	assert.NoError(t, err, "Failed to create generator")

	// Attempting to convert our HCL code
	codeBlock, stderr, err := g.convertHCL(hcl, "EXAMPLE_NAME")

	// Checking for error and printing if it exists
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(stderr)
	}

	// Printing translated code in the case that it was successfully converted
	fmt.Println(codeBlock)

	assert.NoError(t, err)
}
