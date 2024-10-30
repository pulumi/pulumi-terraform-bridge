// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceID(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_id" "foo" {
  							byte_length = 4
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_id.foo", "b64_url", testCheckLen(6)),
					resource.TestCheckResourceAttrWith("random_id.foo", "b64_std", testCheckLen(8)),
					resource.TestCheckResourceAttrWith("random_id.foo", "hex", testCheckLen(8)),
					resource.TestCheckResourceAttrWith("random_id.foo", "dec", testCheckMinLen(1)),
				),
			},
			{
				ResourceName:      "random_id.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccResourceID_ImportWithPrefix(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_id" "bar" {
  							byte_length = 4
  							prefix      = "cloud-"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_id.bar", "b64_url", testCheckLen(12)),
					resource.TestCheckResourceAttrWith("random_id.bar", "b64_std", testCheckLen(14)),
					resource.TestCheckResourceAttrWith("random_id.bar", "hex", testCheckLen(14)),
					resource.TestCheckResourceAttrWith("random_id.bar", "dec", testCheckMinLen(1)),
				),
			},
			{
				ResourceName:        "random_id.bar",
				ImportState:         true,
				ImportStateIdPrefix: "cloud-,",
				ImportStateVerify:   true,
			},
		},
	})
}

func TestAccResourceID_UpgradeFromVersion3_3_2(t *testing.T) {
    t.Parallel()
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "bar" {
  							byte_length = 4
  							prefix      = "cloud-"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_id.bar", "b64_url", testCheckLen(12)),
					resource.TestCheckResourceAttrWith("random_id.bar", "b64_std", testCheckLen(14)),
					resource.TestCheckResourceAttrWith("random_id.bar", "hex", testCheckLen(14)),
					resource.TestCheckResourceAttrWith("random_id.bar", "dec", testCheckMinLen(1)),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "bar" {
  							byte_length = 4
  							prefix      = "cloud-"
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "bar" {
  							byte_length = 4
  							prefix      = "cloud-"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_id.bar", "b64_url", testCheckLen(12)),
					resource.TestCheckResourceAttrWith("random_id.bar", "b64_std", testCheckLen(14)),
					resource.TestCheckResourceAttrWith("random_id.bar", "hex", testCheckLen(14)),
					resource.TestCheckResourceAttrWith("random_id.bar", "dec", testCheckMinLen(1)),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_EmptyMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_EmptyMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_NullMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_NullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_NullValues(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_Value(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Keep_Values(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_EmptyMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_NullMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_NullValueToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_ValueToNullMap(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_ValueToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_Replace_ValueToNewValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_FrameworkMigration_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_FrameworkMigration_NullMapToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_FrameworkMigration_NullMapToMultipleNullValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_FrameworkMigration_NullMapToMultipleValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_FrameworkMigration_NullMapValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceID_Keepers_FrameworkMigration_NullMapValueToValue(t *testing.T) {
    t.Parallel()
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id1),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_id" "test" {
					byte_length = 4
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_id.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_id.test", "keepers.%", "2"),
				),
			},
		},
	})
}
