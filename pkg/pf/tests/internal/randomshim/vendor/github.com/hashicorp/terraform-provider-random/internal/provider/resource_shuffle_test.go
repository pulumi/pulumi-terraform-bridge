// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These results are current as of Go 1.6. The Go
// "rand" package does not guarantee that the random
// number generator will generate the same results
// forever, but the maintainers endeavor not to change
// it gratuitously.
// These tests allow us to detect such changes and
// document them when they arise, but the docs for this
// resource specifically warn that results are not
// guaranteed consistent across Terraform releases.
func TestAccResourceShuffle(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_shuffle" "default_length" {
    						input = ["a", "b", "c", "d", "e"]
    						seed = "-"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.default_length", "result.#", testAccResourceShuffleCheckLength("5")),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.0", "a"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.1", "c"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.2", "b"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.3", "e"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.4", "d"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_EmptyMap(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_EmptyMapToNullValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_NullMap(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_NullValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_NullValues(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_Value(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Keep_Values(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_EmptyMapToValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_NullMapToValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_NullValueToValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_ValueToNullMap(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_ValueToNullValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_Replace_ValueToNewValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_FrameworkMigration_NullMapToNullValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_FrameworkMigration_NullMapToValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_FrameworkMigration_NullMapToMultipleNullValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_FrameworkMigration_NullMapToMultipleValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_FrameworkMigration_NullMapValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Keepers_FrameworkMigration_NullMapValueToValue(t *testing.T) {
    t.Parallel()
	var result1, result2 []string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result1),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "test" {
					input = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"]
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttrList("random_shuffle.test", "result", &result2),
					testCheckAttributeValueListsDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_shuffle.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Shorter(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_shuffle" "shorter_length" {
    						input = ["a", "b", "c", "d", "e"]
    						seed = "-"
    						result_count = 3
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.shorter_length", "result.#", testAccResourceShuffleCheckLength("3")),
					resource.TestCheckResourceAttr("random_shuffle.shorter_length", "result.0", "a"),
					resource.TestCheckResourceAttr("random_shuffle.shorter_length", "result.1", "c"),
					resource.TestCheckResourceAttr("random_shuffle.shorter_length", "result.2", "b"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Longer(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_shuffle" "longer_length" {
    						input = ["a", "b", "c", "d", "e"]
    						seed = "-"
    						result_count = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.longer_length", "result.#", testAccResourceShuffleCheckLength("12")),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.0", "a"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.1", "c"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.2", "b"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.3", "e"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.4", "d"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.5", "a"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.6", "e"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.7", "d"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.8", "c"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.9", "b"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.10", "a"),
					resource.TestCheckResourceAttr("random_shuffle.longer_length", "result.11", "b"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_Empty(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_shuffle" "empty_length" {
    						input = []
    						seed = "-"
    						result_count = 12
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.empty_length", "result.#", testAccResourceShuffleCheckLength("0")),
				),
			},
		},
	})
}

func TestAccResourceShuffle_One(t *testing.T) {
    t.Parallel()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_shuffle" "one_length" {
    						input = ["a"]
    						seed = "-"
    						result_count = 1
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.one_length", "result.#", testAccResourceShuffleCheckLength("1")),
					resource.TestCheckResourceAttr("random_shuffle.one_length", "result.0", "a"),
				),
			},
		},
	})
}

func TestAccResourceShuffle_UpgradeFromVersion3_3_2(t *testing.T) {
    t.Parallel()
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: providerVersion332(),
				Config: `resource "random_shuffle" "default_length" {
    						input = ["a", "b", "c", "d", "e"]
    						seed = "-"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.default_length", "result.#", testAccResourceShuffleCheckLength("5")),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.0", "a"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.1", "c"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.2", "b"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.3", "e"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.4", "d"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "default_length" {
    						input = ["a", "b", "c", "d", "e"]
    						seed = "-"
						}`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_shuffle" "default_length" {
    						input = ["a", "b", "c", "d", "e"]
    						seed = "-"
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("random_shuffle.default_length", "result.#", testAccResourceShuffleCheckLength("5")),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.0", "a"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.1", "c"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.2", "b"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.3", "e"),
					resource.TestCheckResourceAttr("random_shuffle.default_length", "result.4", "d"),
				),
			},
		},
	})
}

func testAccResourceShuffleCheckLength(expectedLength string) func(input string) error {
	return func(input string) error {
		if input != expectedLength {
			return fmt.Errorf("got length %s; expected length %s", input, expectedLength)
		}

		return nil
	}
}

//nolint:unparam
func testExtractResourceAttrList(resourceName string, attributeName string, attributeValue *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]

		if !ok {
			return fmt.Errorf("resource name %s not found in state", resourceName)
		}

		elementCountAttr := attributeName + ".#"

		elementCountValue, ok := rs.Primary.Attributes[elementCountAttr]

		if !ok {
			return fmt.Errorf("attribute %s not found in resource %s state", elementCountAttr, resourceName)
		}

		elementCount, err := strconv.Atoi(elementCountValue)

		if err != nil {
			return fmt.Errorf("attribute %s not integer: %w", elementCountAttr, err)
		}

		listValue := make([]string, elementCount)

		for i := 0; i < elementCount; i++ {
			attr := attributeName + "." + strconv.Itoa(i)

			attrValue, ok := rs.Primary.Attributes[attr]

			if !ok {
				return fmt.Errorf("attribute %s not found in resource %s state", attr, resourceName)
			}

			listValue[i] = attrValue
		}

		*attributeValue = listValue

		return nil
	}
}

func testCheckAttributeValueListsDiffer(i *[]string, j *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if reflect.DeepEqual(i, j) {
			return fmt.Errorf("attribute values are the same")
		}

		return nil
	}
}

func testCheckAttributeValueListsEqual(i *[]string, j *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if !reflect.DeepEqual(i, j) {
			return fmt.Errorf("attribute values are different, got %v and %v", i, j)
		}

		return nil
	}
}
