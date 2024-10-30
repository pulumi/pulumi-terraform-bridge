// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourcePet(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_pet" "pet_1" {
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_pet.pet_1", "id", testCheckPetLen("-", 2)),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_EmptyMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_EmptyMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_NullMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_NullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_NullValues(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_Value(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Keep_Values(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_EmptyMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_NullMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_NullValueToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_ValueToNullMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_ValueToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_Replace_ValueToNewValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_FrameworkMigration_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_FrameworkMigration_NullMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_FrameworkMigration_NullMapToMultipleNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_FrameworkMigration_NullMapToMultipleValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_FrameworkMigration_NullMapValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourcePet_Keepers_FrameworkMigration_NullMapValueToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id1),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "test" {
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_pet.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_pet.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourcePet_Length(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_pet" "pet_1" {
  							length = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_pet.pet_1", "id", testCheckPetLen("-", 4)),
				),
			},
		},
	})
}

func TestAccResourcePet_Prefix(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_pet" "pet_1" {
  							prefix = "consul"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_pet.pet_1", "id", testCheckPetLen("-", 3)),
					resource.TestMatchResourceAttr("random_pet.pet_1", "id", regexp.MustCompile("^consul-")),
				),
			},
		},
	})
}

func TestAccResourcePet_Separator(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_pet" "pet_1" {
  							length = 3
  							separator = "_"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_pet.pet_1", "id", testCheckPetLen("_", 3)),
				),
			},
		},
	})
}

func TestAccResourcePet_UpgradeFromVersion3_3_2(t *testing.T) {
    t.Parallel()
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_pet" "pet_1" {
  							prefix = "consul"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_pet.pet_1", "id", testCheckPetLen("-", 3)),
					resource.TestMatchResourceAttr("random_pet.pet_1", "id", regexp.MustCompile("^consul-")),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "pet_1" {
  							prefix = "consul"
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_pet" "pet_1" {
  							prefix = "consul"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_pet.pet_1", "id", testCheckPetLen("-", 3)),
					resource.TestMatchResourceAttr("random_pet.pet_1", "id", regexp.MustCompile("^consul-")),
				),
			},
		},
	})
}

func testCheckPetLen(separator string, expectedLen int) func(input string) error {
	return func(input string) error {
		petNameParts := strings.Split(input, separator)

		if len(petNameParts) != expectedLen {
			return fmt.Errorf("expected length %d, actual length %d", expectedLen, len(petNameParts))
		}

		return nil
	}
}
