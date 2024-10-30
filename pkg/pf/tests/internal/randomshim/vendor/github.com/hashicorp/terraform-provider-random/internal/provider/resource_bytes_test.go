// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceBytes(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_bytes" "basic" {
							length = 32
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("random_bytes.basic", "base64", regexp.MustCompile(`^[A-Za-z/+\d]{43}=$`)),
					resource.TestMatchResourceAttr("random_bytes.basic", "hex", regexp.MustCompile(`^[a-f\d]{64}$`)),
					resource.TestCheckResourceAttr("random_bytes.basic", "length", "32"),
				),
			},
			{
				// Usage of ImportStateIdFunc is required as the value passed to the `terraform import` command needs
				// to be the bytes encoded with base64.
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					resource := "random_bytes.basic"
					rs, ok := s.RootModule().Resources[resource]
					if !ok {
						return "", fmt.Errorf("not found: %s", resource)
					}
					if rs.Primary.Attributes["base64"] == "" {
						return "", fmt.Errorf("no base64 attribute is set")
					}

					return rs.Primary.Attributes["base64"], nil
				},
				ResourceName:                         "random_bytes.basic",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "base64",
			},
		},
	})
}

func TestAccResourceBytes_ImportWithoutKeepersThenUpdateShouldNotTriggerChange(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				ImportState:        true,
				ImportStateId:      "hkvbcU5f8qGysTFhkI4gzf3yRWC1jXW3aRLCNQFOtNw=",
				ImportStatePersist: true,
				ResourceName:       "random_bytes.basic",
				Config: `resource "random_bytes" "basic" {
							length = 32
						}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_bytes.basic", "base64", "hkvbcU5f8qGysTFhkI4gzf3yRWC1jXW3aRLCNQFOtNw="),
					resource.TestCheckResourceAttr("random_bytes.basic", "hex", "864bdb714e5ff2a1b2b13161908e20cdfdf24560b58d75b76912c235014eb4dc"),
					resource.TestCheckResourceAttr("random_bytes.basic", "length", "32"),
				),
			},
			{
				Config: `resource "random_bytes" "basic" {
							length = 32
						}`,
				PlanOnly: true,
			},
		},
	})
}

func TestAccResourceBytes_LengthErrors(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `resource "random_bytes" "invalid_length" {
							length = 0
						}`,
				ExpectError: regexp.MustCompile(`.*Attribute length value must be at least 1, got: 0`),
			},
		},
	})
}

func TestAccResourceBytes_Length_ForceReplacement(t *testing.T) {
	var bytes1, bytes2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 1
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_bytes.test", "length", "1"),
					testExtractResourceAttr("random_bytes.test", "base64", &bytes1),
					resource.TestCheckResourceAttrWith("random_bytes.test", "hex", testCheckLen(2)),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 2
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("random_bytes.test", "length", "2"),
					testExtractResourceAttr("random_bytes.test", "base64", &bytes2),
					resource.TestCheckResourceAttrWith("random_bytes.test", "hex", testCheckLen(4)),
					testCheckAttributeValuesDiffer(&bytes1, &bytes2),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Keep_EmptyMap(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Keep_NullMap(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Keep_NullValue(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Keep_NullValues(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key1" = null
						"key2" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Keep_Value(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Keep_Values(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "2"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key1" = "123"
						"key2" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesEqual(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "2"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_EmptyMapToValue(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_NullMapToValue(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_NullValueToValue(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_ValueToEmptyMap(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_ValueToNullMap(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "0"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_ValueToNullValue(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = null
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}

func TestAccResourceBytes_Keepers_Replace_ValueToNewValue(t *testing.T) {
	var result1, result2 string

	resource.ParallelTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "123"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result1),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `resource "random_bytes" "test" {
					length = 12
					keepers = {
						"key" = "456"
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					testExtractResourceAttr("random_bytes.test", "hex", &result2),
					testCheckAttributeValuesDiffer(&result1, &result2),
					resource.TestCheckResourceAttr("random_bytes.test", "keepers.%", "1"),
				),
			},
		},
	})
}
