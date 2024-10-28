// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccResourceInteger(t *testing.T) {
	t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
							max  = 3
   							seed = "12345"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "3"),
				),
			},
			{
				ResourceName:      "random_integer.integer_1",
				ImportState:       true,
				ImportStateId:     "3,1,3,12345",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccResourceInteger_ChangeSeed(t *testing.T) {
	t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
							max  = 3
   							seed = "12345"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "3"),
				),
			},
			{
				Config: `resource "random_integer" "integer_1" {
							min  = 1
   							max  = 3
   							seed = "123456"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_SeedlessToSeeded(t *testing.T) {
	t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
   							max  = 3
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_integer.integer_1", "result", testCheckNotEmptyString("result")),
				),
			},
			{
				Config: `resource "random_integer" "integer_1" {
							min  = 1
   							max  = 3
   							seed = "123456"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_SeededToSeedless(t *testing.T) {
	t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
							max  = 3
   							seed = "12345"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "3"),
				),
			},
			{
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
   							max  = 3
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_integer.integer_1", "result", testCheckNotEmptyString("result")),
				),
			},
		},
	})
}

func TestAccResourceInteger_Big(t *testing.T) {
	t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_integer" "integer_1" {
   							max  = 7227701560655103598
   							min  = 7227701560655103597
   							seed = 12345
						}`,
			},
			{
				ResourceName:      "random_integer.integer_1",
				ImportState:       true,
				ImportStateId:     "7227701560655103598,7227701560655103597,7227701560655103598,12345",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccResourceInteger_UpgradeFromVersion3_3_2(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
							max  = 3
   							seed = "12345"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "3"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
							max  = 3
   							seed = "12345"
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "integer_1" {
   							min  = 1
							max  = 3
   							seed = "12345"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_integer.integer_1", "result", "3"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_EmptyMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_EmptyMapToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_NullMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_NullMapToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_NullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_NullValues(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_Value(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Keep_Values(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_EmptyMapToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_NullMapToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_NullValueToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_ValueToNullMap(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_ValueToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_Replace_ValueToNewValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_FrameworkMigration_NullMapToNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_FrameworkMigration_NullMapToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_FrameworkMigration_NullMapToMultipleNullValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_FrameworkMigration_NullMapToMultipleValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_FrameworkMigration_NullMapValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesEqual(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceInteger_Keepers_FrameworkMigration_NullMapValueToValue(t *testing.T) {
	var id1, id2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id1),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_integer" "test" {
					min = 1
					max = 100000000
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_integer.test", "id", &id2),
					testCheckAttributeValuesDiffer(&id1, &id2),
					resource.TestCheckResourceAttr("random_integer.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func testCheckNotEmptyString(field string) func(input string) error {
	return func(input string) error {
		if input == "" {
			return fmt.Errorf("%s is empty string", field)
		}

		return nil
	}
}

func testExtractResourceAttr(resourceName string, attributeName string, attributeValue *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]

		if !ok {
			return fmt.Errorf("resource name %s not found in state", resourceName)
		}

		attrValue, ok := rs.Primary.Attributes[attributeName]

		if !ok {
			return fmt.Errorf("attribute %s not found in resource %s state", attributeName, resourceName)
		}

		*attributeValue = attrValue

		return nil
	}
}

func testCheckAttributeValuesDiffer(i *string, j *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if testStringValue(i) == testStringValue(j) {
			return fmt.Errorf("attribute values are the same")
		}

		return nil
	}
}

func testCheckAttributeValuesEqual(i *string, j *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if testStringValue(i) != testStringValue(j) {
			return fmt.Errorf("attribute values are different, got %s and %s", testStringValue(i), testStringValue(j))
		}

		return nil
	}
}

func testStringValue(sPtr *string) string {
	if sPtr == nil {
		return ""
	}

	return *sPtr
}
