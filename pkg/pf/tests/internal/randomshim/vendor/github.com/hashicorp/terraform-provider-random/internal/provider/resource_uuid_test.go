// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceUUID(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_uuid" "basic" { 
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("random_uuid.basic", "result", regexp.MustCompile(`[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}`)),
				),
			},
			{
				ResourceName:      "random_uuid.basic",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_EmptyMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_EmptyMapToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_NullMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_NullMapToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_NullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_NullValues(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_Value(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Keep_Values(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_EmptyMapToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_NullMapToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_NullValueToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_ValueToNullMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_ValueToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_Replace_ValueToNewValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_FrameworkMigration_NullMapToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_FrameworkMigration_NullMapToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_FrameworkMigration_NullMapToMultipleNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_FrameworkMigration_NullMapToMultipleValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_FrameworkMigration_NullMapValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceUUID_Keepers_FrameworkMigration_NullMapValueToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id1),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_uuid.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_uuid.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceUUID_UpgradeFromVersion3_3_2(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_uuid" "basic" { 
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("random_uuid.basic", "result", regexp.MustCompile(`[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}`)),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "basic" { 
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_uuid" "basic" { 
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("random_uuid.basic", "result", regexp.MustCompile(`[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}`)),
				),
			},
		},
	})
}
