// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	res "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceString_Import(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_string" "basic" {
							length = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.basic", "result", testCheckLen(12)),
				),
			},
			{
				ResourceName:      "random_string.basic",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_EmptyMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_EmptyMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_NullMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_NullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_NullValues(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_Value(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Keep_Values(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_EmptyMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_NullMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_NullValueToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_ValueToNullMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_ValueToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_Replace_ValueToNewValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_FrameworkMigration_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_FrameworkMigration_NullMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_FrameworkMigration_NullMapToMultipleNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_FrameworkMigration_NullMapToMultipleValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_FrameworkMigration_NullMapValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceString_Keepers_FrameworkMigration_NullMapValueToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id1),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_string.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_string.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceString_Override(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_string" "override" {
							length = 4
							override_special = "!"
							lower = false
							upper = false
							numeric = false
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.override", "result", testCheckLen(4)),
					resource.TestCheckResourceAttr("random_string.override", "result", "!!!!"),
				),
			},
		},
	})
}

// TestAccResourceString_OverrideSpecial_FromVersion3_3_2 verifies behaviour
// when upgrading the provider version from 3.3.2, which set the
// override_special value to null and should not result in a plan difference.
// Reference: https://github.com/hashicorp/terraform-provider-random/issues/306
func TestAccResourceString_OverrideSpecial_FromVersion3_3_2(t *testing.T) {
    t.Parallel()
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "test" {
							length = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("random_string.test", "override_special"),
					testExtractResourceAttr("random_string.test", "result", &result1),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("random_string.test", "override_special"),
					testExtractResourceAttr("random_string.test", "result", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
				),
			},
		},
	})
}

// TestAccResourceString_OverrideSpecial_FromVersion3_4_2 verifies behaviour
// when upgrading the provider version from 3.4.2, which set the
// override_special value to "", while other versions do not.
// Reference: https://github.com/hashicorp/terraform-provider-random/issues/306
func TestAccResourceString_OverrideSpecial_FromVersion3_4_2(t *testing.T) {
    t.Parallel()
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion342(),
				Config: `resource "random_string" "test" {
							length = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_string.test", "override_special", ""),
					testExtractResourceAttr("random_string.test", "result", &result1),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("random_string.test", "override_special"),
					testExtractResourceAttr("random_string.test", "result", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
				),
			},
		},
	})
}

func TestAccResourceString_Min(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.min", "result", testCheckLen(12)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([a-z].*){2,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([A-Z].*){3,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([0-9].*){4,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([!#@].*)`)),
				),
			},
		},
	})
}

// TestAccResourceString_StateUpgradeV1toV2 covers the state upgrade from V1 to V2.
// This includes the deprecation of `number` and the addition of `numeric` attributes.
// v3.2.0 was used as this is the last version before `number` was deprecated and `numeric` attribute
// was added.
func TestAccResourceString_StateUpgradeV1toV2(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		configBeforeUpgrade string
		configDuringUpgrade string
		beforeStateUpgrade  []resource.TestCheckFunc
		afterStateUpgrade   []resource.TestCheckFunc
	}{
		{
			name: "number is absent before number and numeric are absent during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is absent before numeric is true during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
						numeric = true
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is absent before numeric is false during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
						numeric = false
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "false"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is true before numeric is true during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
						number = true
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
						numeric = true
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is true before number and numeric are absent during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
						number = true
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is true before numeric is false during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
						number = true
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
						numeric = false
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "false"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is false before numeric is false during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
						number = false
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
						numeric = false
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "false"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "false"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is false before number and numeric are absent during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
						number = false
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "false"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
		{
			name: "number is false before numeric is true during",
			configBeforeUpgrade: `resource "random_string" "default" {
						length = 12
						number = false
					}`,
			configDuringUpgrade: `resource "random_string" "default" {
						length = 12
						numeric = true
					}`,
			beforeStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "false"),
				resource.TestCheckNoResourceAttr("random_string.default", "numeric"),
			},
			afterStateUpgrade: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("random_string.default", "number", "true"),
				resource.TestCheckResourceAttrPair("random_string.default", "number", "random_string.default", "numeric"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				Steps: []resource.TestStep{
					{
						ExternalProviders: map[string]resource.ExternalProvider{"random": {
							VersionConstraint: "3.2.0",
							Source:            "hashicorp/random",
						}},
						Config: c.configBeforeUpgrade,
						Check:  resource.ComposeTestCheckFunc(c.beforeStateUpgrade...),
					},
					{
						ProtoV5ProviderFactories: protoV5ProviderFactories(),
						Config:                   c.configDuringUpgrade,
						Check:                    resource.ComposeTestCheckFunc(c.afterStateUpgrade...),
					},
				},
			})
		})
	}
}

func TestAccResourceString_LengthErrors(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_string" "invalid_length" {
  							length = 2
  							min_lower = 3
						}`,
				ExpectError: regexp.MustCompile(`.*Attribute length value must be at least sum of min_upper \+ min_lower \+\nmin_numeric \+ min_special, got: 2`),
			},
			{
				Config: `resource "random_string" "invalid_length" {
							length = 0
						}`,
				ExpectError: regexp.MustCompile(`.*Attribute length value must be at least 1, got: 0`),
			},
		},
	})
}

// TestAccResourceString_UpgradeFromVersion3_2_0 verifies behaviour when upgrading state from schema V1 to V2.
func TestAccResourceString_UpgradeFromVersion3_2_0(t *testing.T) {
    t.Parallel()
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion320(),
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.min", "result", testCheckLen(12)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([a-z].*){2,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([A-Z].*){3,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([0-9].*){4,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([!#@])`)),
					resource.TestCheckResourceAttr("random_string.min", "special", "true"),
					resource.TestCheckResourceAttr("random_string.min", "upper", "true"),
					resource.TestCheckResourceAttr("random_string.min", "lower", "true"),
					resource.TestCheckResourceAttr("random_string.min", "number", "true"),
					resource.TestCheckResourceAttr("random_string.min", "min_special", "1"),
					resource.TestCheckResourceAttr("random_string.min", "min_upper", "3"),
					resource.TestCheckResourceAttr("random_string.min", "min_lower", "2"),
					resource.TestCheckResourceAttr("random_string.min", "min_numeric", "4"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.min", "result", testCheckLen(12)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([a-z].*){2,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([A-Z].*){3,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([0-9].*){4,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([!#@])`)),
					resource.TestCheckResourceAttr("random_string.min", "special", "true"),
					resource.TestCheckResourceAttr("random_string.min", "upper", "true"),
					resource.TestCheckResourceAttr("random_string.min", "lower", "true"),
					resource.TestCheckResourceAttr("random_string.min", "number", "true"),
					resource.TestCheckResourceAttr("random_string.min", "numeric", "true"),
					resource.TestCheckResourceAttr("random_string.min", "min_special", "1"),
					resource.TestCheckResourceAttr("random_string.min", "min_upper", "3"),
					resource.TestCheckResourceAttr("random_string.min", "min_lower", "2"),
					resource.TestCheckResourceAttr("random_string.min", "min_numeric", "4"),
				),
			},
		},
	})
}

// TestAccResourceString_UpgradeFromVersion3_3_2 verifies behaviour when upgrading from SDKv2 to the Framework.
func TestAccResourceString_UpgradeFromVersion3_3_2(t *testing.T) {
    t.Parallel()
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.min", "result", testCheckLen(12)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([a-z].*){2,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([A-Z].*){3,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([0-9].*){4,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([!#@])`)),
					resource.TestCheckResourceAttr("random_string.min", "special", "true"),
					resource.TestCheckResourceAttr("random_string.min", "upper", "true"),
					resource.TestCheckResourceAttr("random_string.min", "lower", "true"),
					resource.TestCheckResourceAttr("random_string.min", "number", "true"),
					resource.TestCheckResourceAttr("random_string.min", "numeric", "true"),
					resource.TestCheckResourceAttr("random_string.min", "min_special", "1"),
					resource.TestCheckResourceAttr("random_string.min", "min_upper", "3"),
					resource.TestCheckResourceAttr("random_string.min", "min_lower", "2"),
					resource.TestCheckResourceAttr("random_string.min", "min_numeric", "4"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "min" {
							length = 12
							override_special = "!#@"
							min_lower = 2
							min_upper = 3
							min_special = 1
							min_numeric = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.min", "result", testCheckLen(12)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([a-z].*){2,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([A-Z].*){3,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([0-9].*){4,}`)),
					resource.TestMatchResourceAttr("random_string.min", "result", regexp.MustCompile(`([!#@])`)),
					resource.TestCheckResourceAttr("random_string.min", "special", "true"),
					resource.TestCheckResourceAttr("random_string.min", "upper", "true"),
					resource.TestCheckResourceAttr("random_string.min", "lower", "true"),
					resource.TestCheckResourceAttr("random_string.min", "number", "true"),
					resource.TestCheckResourceAttr("random_string.min", "numeric", "true"),
					resource.TestCheckResourceAttr("random_string.min", "min_special", "1"),
					resource.TestCheckResourceAttr("random_string.min", "min_upper", "3"),
					resource.TestCheckResourceAttr("random_string.min", "min_lower", "2"),
					resource.TestCheckResourceAttr("random_string.min", "min_numeric", "4"),
				),
			},
		},
	})
}

func TestUpgradeStringStateV1toV3(t *testing.T) {
	t.Parallel()

	req := res.UpgradeStateRequest{
		State: &tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":               tftypes.String,
					"keepers":          tftypes.Map{ElementType: tftypes.String},
					"length":           tftypes.Number,
					"lower":            tftypes.Bool,
					"min_lower":        tftypes.Number,
					"min_numeric":      tftypes.Number,
					"min_special":      tftypes.Number,
					"min_upper":        tftypes.Number,
					"number":           tftypes.Bool,
					"override_special": tftypes.String,
					"result":           tftypes.String,
					"special":          tftypes.Bool,
					"upper":            tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"id":               tftypes.NewValue(tftypes.String, "none"),
				"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"length":           tftypes.NewValue(tftypes.Number, 16),
				"lower":            tftypes.NewValue(tftypes.Bool, true),
				"min_lower":        tftypes.NewValue(tftypes.Number, 0),
				"min_numeric":      tftypes.NewValue(tftypes.Number, 0),
				"min_special":      tftypes.NewValue(tftypes.Number, 0),
				"min_upper":        tftypes.NewValue(tftypes.Number, 0),
				"number":           tftypes.NewValue(tftypes.Bool, true),
				"override_special": tftypes.NewValue(tftypes.String, "!#$%\u0026*()-_=+[]{}\u003c\u003e:?"),
				"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
				"special":          tftypes.NewValue(tftypes.Bool, true),
				"upper":            tftypes.NewValue(tftypes.Bool, true),
			}),
			Schema: stringSchemaV1(),
		},
	}

	resp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Schema: stringSchemaV3(),
		},
	}

	upgradeStringStateV1toV3(context.Background(), req, resp)

	expectedResp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":               tftypes.String,
					"keepers":          tftypes.Map{ElementType: tftypes.String},
					"length":           tftypes.Number,
					"lower":            tftypes.Bool,
					"min_lower":        tftypes.Number,
					"min_numeric":      tftypes.Number,
					"min_special":      tftypes.Number,
					"min_upper":        tftypes.Number,
					"number":           tftypes.Bool,
					"numeric":          tftypes.Bool,
					"override_special": tftypes.String,
					"result":           tftypes.String,
					"special":          tftypes.Bool,
					"upper":            tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"id":               tftypes.NewValue(tftypes.String, "none"),
				"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"length":           tftypes.NewValue(tftypes.Number, 16),
				"lower":            tftypes.NewValue(tftypes.Bool, true),
				"min_lower":        tftypes.NewValue(tftypes.Number, 0),
				"min_numeric":      tftypes.NewValue(tftypes.Number, 0),
				"min_special":      tftypes.NewValue(tftypes.Number, 0),
				"min_upper":        tftypes.NewValue(tftypes.Number, 0),
				"number":           tftypes.NewValue(tftypes.Bool, true),
				"numeric":          tftypes.NewValue(tftypes.Bool, true),
				"override_special": tftypes.NewValue(tftypes.String, "!#$%\u0026*()-_=+[]{}\u003c\u003e:?"),
				"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
				"special":          tftypes.NewValue(tftypes.Bool, true),
				"upper":            tftypes.NewValue(tftypes.Bool, true),
			}),
			Schema: stringSchemaV3(),
		},
	}

	if !cmp.Equal(expectedResp, resp) {
		t.Errorf("expected: %+v, got: %+v", expectedResp, resp)
	}
}

func TestUpgradeStringStateV1toV3_NullValues(t *testing.T) {
	t.Parallel()

	raw := tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":               tftypes.String,
			"keepers":          tftypes.Map{ElementType: tftypes.String},
			"length":           tftypes.Number,
			"lower":            tftypes.Bool,
			"min_lower":        tftypes.Number,
			"min_numeric":      tftypes.Number,
			"min_special":      tftypes.Number,
			"min_upper":        tftypes.Number,
			"number":           tftypes.Bool,
			"override_special": tftypes.String,
			"result":           tftypes.String,
			"special":          tftypes.Bool,
			"upper":            tftypes.Bool,
		},
	}, map[string]tftypes.Value{
		"id":               tftypes.NewValue(tftypes.String, "none"),
		"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
		"length":           tftypes.NewValue(tftypes.Number, nil),
		"lower":            tftypes.NewValue(tftypes.Bool, nil),
		"min_lower":        tftypes.NewValue(tftypes.Number, nil),
		"min_numeric":      tftypes.NewValue(tftypes.Number, nil),
		"min_special":      tftypes.NewValue(tftypes.Number, nil),
		"min_upper":        tftypes.NewValue(tftypes.Number, nil),
		"number":           tftypes.NewValue(tftypes.Bool, nil),
		"override_special": tftypes.NewValue(tftypes.String, nil),
		"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
		"special":          tftypes.NewValue(tftypes.Bool, nil),
		"upper":            tftypes.NewValue(tftypes.Bool, nil),
	})

	req := res.UpgradeStateRequest{
		State: &tfsdk.State{
			Raw:    raw,
			Schema: stringSchemaV1(),
		},
	}

	resp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Schema: stringSchemaV3(),
		},
	}

	upgradeStringStateV1toV3(context.Background(), req, resp)

	expectedResp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":               tftypes.String,
					"keepers":          tftypes.Map{ElementType: tftypes.String},
					"length":           tftypes.Number,
					"lower":            tftypes.Bool,
					"min_lower":        tftypes.Number,
					"min_numeric":      tftypes.Number,
					"min_special":      tftypes.Number,
					"min_upper":        tftypes.Number,
					"number":           tftypes.Bool,
					"numeric":          tftypes.Bool,
					"override_special": tftypes.String,
					"result":           tftypes.String,
					"special":          tftypes.Bool,
					"upper":            tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"id":               tftypes.NewValue(tftypes.String, "none"),
				"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"length":           tftypes.NewValue(tftypes.Number, 16),
				"lower":            tftypes.NewValue(tftypes.Bool, true),
				"min_lower":        tftypes.NewValue(tftypes.Number, 0),
				"min_numeric":      tftypes.NewValue(tftypes.Number, 0),
				"min_special":      tftypes.NewValue(tftypes.Number, 0),
				"min_upper":        tftypes.NewValue(tftypes.Number, 0),
				"number":           tftypes.NewValue(tftypes.Bool, true),
				"numeric":          tftypes.NewValue(tftypes.Bool, true),
				"override_special": tftypes.NewValue(tftypes.String, nil),
				"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
				"special":          tftypes.NewValue(tftypes.Bool, true),
				"upper":            tftypes.NewValue(tftypes.Bool, true),
			}),
			Schema: stringSchemaV3(),
		},
	}

	if !cmp.Equal(expectedResp, resp) {
		t.Errorf("expected: %+v, got: %+v", expectedResp, resp)
	}
}

func TestUpgradeStringStateV2toV3(t *testing.T) {
	t.Parallel()

	req := res.UpgradeStateRequest{
		State: &tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":               tftypes.String,
					"keepers":          tftypes.Map{ElementType: tftypes.String},
					"length":           tftypes.Number,
					"lower":            tftypes.Bool,
					"min_lower":        tftypes.Number,
					"min_numeric":      tftypes.Number,
					"min_special":      tftypes.Number,
					"min_upper":        tftypes.Number,
					"number":           tftypes.Bool,
					"numeric":          tftypes.Bool,
					"override_special": tftypes.String,
					"result":           tftypes.String,
					"special":          tftypes.Bool,
					"upper":            tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"id":               tftypes.NewValue(tftypes.String, "none"),
				"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"length":           tftypes.NewValue(tftypes.Number, 16),
				"lower":            tftypes.NewValue(tftypes.Bool, true),
				"min_lower":        tftypes.NewValue(tftypes.Number, 0),
				"min_numeric":      tftypes.NewValue(tftypes.Number, 0),
				"min_special":      tftypes.NewValue(tftypes.Number, 0),
				"min_upper":        tftypes.NewValue(tftypes.Number, 0),
				"number":           tftypes.NewValue(tftypes.Bool, true),
				"numeric":          tftypes.NewValue(tftypes.Bool, true),
				"override_special": tftypes.NewValue(tftypes.String, "!#$%\u0026*()-_=+[]{}\u003c\u003e:?"),
				"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
				"special":          tftypes.NewValue(tftypes.Bool, true),
				"upper":            tftypes.NewValue(tftypes.Bool, true),
			}),
			Schema: stringSchemaV2(),
		},
	}

	resp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Schema: stringSchemaV3(),
		},
	}

	upgradeStringStateV2toV3(context.Background(), req, resp)

	expectedResp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":               tftypes.String,
					"keepers":          tftypes.Map{ElementType: tftypes.String},
					"length":           tftypes.Number,
					"lower":            tftypes.Bool,
					"min_lower":        tftypes.Number,
					"min_numeric":      tftypes.Number,
					"min_special":      tftypes.Number,
					"min_upper":        tftypes.Number,
					"number":           tftypes.Bool,
					"numeric":          tftypes.Bool,
					"override_special": tftypes.String,
					"result":           tftypes.String,
					"special":          tftypes.Bool,
					"upper":            tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"id":               tftypes.NewValue(tftypes.String, "none"),
				"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"length":           tftypes.NewValue(tftypes.Number, 16),
				"lower":            tftypes.NewValue(tftypes.Bool, true),
				"min_lower":        tftypes.NewValue(tftypes.Number, 0),
				"min_numeric":      tftypes.NewValue(tftypes.Number, 0),
				"min_special":      tftypes.NewValue(tftypes.Number, 0),
				"min_upper":        tftypes.NewValue(tftypes.Number, 0),
				"number":           tftypes.NewValue(tftypes.Bool, true),
				"numeric":          tftypes.NewValue(tftypes.Bool, true),
				"override_special": tftypes.NewValue(tftypes.String, "!#$%\u0026*()-_=+[]{}\u003c\u003e:?"),
				"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
				"special":          tftypes.NewValue(tftypes.Bool, true),
				"upper":            tftypes.NewValue(tftypes.Bool, true),
			}),
			Schema: stringSchemaV3(),
		},
	}

	if !cmp.Equal(expectedResp, resp) {
		t.Errorf("expected: %+v, got: %+v", expectedResp, resp)
	}
}

func TestUpgradeStringStateV2toV3_NullValues(t *testing.T) {
	t.Parallel()

	raw := tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":               tftypes.String,
			"keepers":          tftypes.Map{ElementType: tftypes.String},
			"length":           tftypes.Number,
			"lower":            tftypes.Bool,
			"min_lower":        tftypes.Number,
			"min_numeric":      tftypes.Number,
			"min_special":      tftypes.Number,
			"min_upper":        tftypes.Number,
			"number":           tftypes.Bool,
			"numeric":          tftypes.Bool,
			"override_special": tftypes.String,
			"result":           tftypes.String,
			"special":          tftypes.Bool,
			"upper":            tftypes.Bool,
		},
	}, map[string]tftypes.Value{
		"id":               tftypes.NewValue(tftypes.String, "none"),
		"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
		"length":           tftypes.NewValue(tftypes.Number, nil),
		"lower":            tftypes.NewValue(tftypes.Bool, nil),
		"min_lower":        tftypes.NewValue(tftypes.Number, nil),
		"min_numeric":      tftypes.NewValue(tftypes.Number, nil),
		"min_special":      tftypes.NewValue(tftypes.Number, nil),
		"min_upper":        tftypes.NewValue(tftypes.Number, nil),
		"number":           tftypes.NewValue(tftypes.Bool, nil),
		"numeric":          tftypes.NewValue(tftypes.Bool, nil),
		"override_special": tftypes.NewValue(tftypes.String, nil),
		"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
		"special":          tftypes.NewValue(tftypes.Bool, nil),
		"upper":            tftypes.NewValue(tftypes.Bool, nil),
	})

	req := res.UpgradeStateRequest{
		State: &tfsdk.State{
			Raw:    raw,
			Schema: stringSchemaV2(),
		},
	}

	resp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Schema: stringSchemaV3(),
		},
	}

	upgradeStringStateV2toV3(context.Background(), req, resp)

	expectedResp := &res.UpgradeStateResponse{
		State: tfsdk.State{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":               tftypes.String,
					"keepers":          tftypes.Map{ElementType: tftypes.String},
					"length":           tftypes.Number,
					"lower":            tftypes.Bool,
					"min_lower":        tftypes.Number,
					"min_numeric":      tftypes.Number,
					"min_special":      tftypes.Number,
					"min_upper":        tftypes.Number,
					"number":           tftypes.Bool,
					"numeric":          tftypes.Bool,
					"override_special": tftypes.String,
					"result":           tftypes.String,
					"special":          tftypes.Bool,
					"upper":            tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"id":               tftypes.NewValue(tftypes.String, "none"),
				"keepers":          tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"length":           tftypes.NewValue(tftypes.Number, 16),
				"lower":            tftypes.NewValue(tftypes.Bool, true),
				"min_lower":        tftypes.NewValue(tftypes.Number, 0),
				"min_numeric":      tftypes.NewValue(tftypes.Number, 0),
				"min_special":      tftypes.NewValue(tftypes.Number, 0),
				"min_upper":        tftypes.NewValue(tftypes.Number, 0),
				"number":           tftypes.NewValue(tftypes.Bool, true),
				"numeric":          tftypes.NewValue(tftypes.Bool, true),
				"override_special": tftypes.NewValue(tftypes.String, nil),
				"result":           tftypes.NewValue(tftypes.String, "DZy_3*tnonj%Q%Yx"),
				"special":          tftypes.NewValue(tftypes.Bool, true),
				"upper":            tftypes.NewValue(tftypes.Bool, true),
			}),
			Schema: stringSchemaV3(),
		},
	}

	if !cmp.Equal(expectedResp, resp) {
		t.Errorf("expected: %+v, got: %+v", expectedResp, resp)
	}
}

// TestAccResourcePassword_String_FromVersion3_1_3 verifies behaviour when resource has been imported and stores
// null for length, lower, number, special, upper, min_lower, min_numeric, min_special, min_upper attributes in state.
// v3.1.3 was selected as this is the last provider version using schema version 1.
func TestAccResourceString_Import_FromVersion3_1_3(t *testing.T) {
    t.Parallel()
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion313(),
				Config: `resource "random_string" "test" {
							length = 12
						}`,
				ResourceName:       "random_string.test",
				ImportState:        true,
				ImportStateId:      "Z=:cbrJE?Ltg",
				ImportStatePersist: true,
				ImportStateCheck: composeImportStateCheck(
					testCheckNoResourceAttrInstanceState("length"),
					testCheckNoResourceAttrInstanceState("number"),
					testCheckNoResourceAttrInstanceState("upper"),
					testCheckNoResourceAttrInstanceState("lower"),
					testCheckNoResourceAttrInstanceState("special"),
					testCheckNoResourceAttrInstanceState("min_numeric"),
					testCheckNoResourceAttrInstanceState("min_upper"),
					testCheckNoResourceAttrInstanceState("min_lower"),
					testCheckNoResourceAttrInstanceState("min_special"),
					testExtractResourceAttrInstanceState("result", &result1),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
					length = 12
				}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
							length = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.test", "result", testCheckLen(12)),
					resource.TestCheckResourceAttr("random_string.test", "number", "true"),
					resource.TestCheckResourceAttr("random_string.test", "numeric", "true"),
					resource.TestCheckResourceAttr("random_string.test", "upper", "true"),
					resource.TestCheckResourceAttr("random_string.test", "lower", "true"),
					resource.TestCheckResourceAttr("random_string.test", "special", "true"),
					resource.TestCheckResourceAttr("random_string.test", "min_numeric", "0"),
					resource.TestCheckResourceAttr("random_string.test", "min_upper", "0"),
					resource.TestCheckResourceAttr("random_string.test", "min_lower", "0"),
					resource.TestCheckResourceAttr("random_string.test", "min_special", "0"),
					testExtractResourceAttr("random_string.test", "result", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
				),
			},
		},
	})
}

// TestAccResourceString_Import_FromVersion3_4_2 verifies behaviour when resource has been imported and stores
// empty map {} for keepers and empty string for override_special in state.
// v3.4.2 was selected as this is the last provider version using schema version 2.
func TestAccResourceString_Import_FromVersion3_4_2(t *testing.T) {
    t.Parallel()
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion342(),
				Config: `resource "random_string" "test" {
							length = 12
						}`,
				ResourceName:       "random_string.test",
				ImportState:        true,
				ImportStateId:      "Z=:cbrJE?Ltg",
				ImportStatePersist: true,
				ImportStateCheck: composeImportStateCheck(
					testCheckResourceAttrInstanceState("length"),
					testCheckResourceAttrInstanceState("number"),
					testCheckResourceAttrInstanceState("numeric"),
					testCheckResourceAttrInstanceState("upper"),
					testCheckResourceAttrInstanceState("lower"),
					testCheckResourceAttrInstanceState("special"),
					testCheckResourceAttrInstanceState("min_numeric"),
					testCheckResourceAttrInstanceState("min_upper"),
					testCheckResourceAttrInstanceState("min_lower"),
					testCheckResourceAttrInstanceState("min_special"),
					testExtractResourceAttrInstanceState("result", &result1),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_string" "test" {
							length = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_string.test", "result", testCheckLen(12)),
					resource.TestCheckResourceAttr("random_string.test", "number", "true"),
					resource.TestCheckResourceAttr("random_string.test", "numeric", "true"),
					resource.TestCheckResourceAttr("random_string.test", "upper", "true"),
					resource.TestCheckResourceAttr("random_string.test", "lower", "true"),
					resource.TestCheckResourceAttr("random_string.test", "special", "true"),
					resource.TestCheckResourceAttr("random_string.test", "min_numeric", "0"),
					resource.TestCheckResourceAttr("random_string.test", "min_upper", "0"),
					resource.TestCheckResourceAttr("random_string.test", "min_lower", "0"),
					resource.TestCheckResourceAttr("random_string.test", "min_special", "0"),
					testExtractResourceAttr("random_string.test", "result", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
				),
			},
		},
	})
}

func testCheckLen(expectedLen int) func(input string) error {
	return func(input string) error {
		if len(input) != expectedLen {
			return fmt.Errorf("expected length %d, actual length %d", expectedLen, len(input))
		}

		return nil
	}
}

//nolint:unparam
func testCheckMinLen(minLen int) func(input string) error {
	return func(input string) error {
		if len(input) < minLen {
			return fmt.Errorf("minimum length %d, actual length %d", minLen, len(input))
		}

		return nil
	}
}
