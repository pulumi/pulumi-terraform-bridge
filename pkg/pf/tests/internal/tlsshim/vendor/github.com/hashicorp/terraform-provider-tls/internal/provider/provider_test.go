package provider

import (
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func setTimeForTest(timeStr string) func() {
	return func() {
		overridableTimeFunc = func() time.Time {
			t, _ := time.Parse(time.RFC3339, timeStr)
			return t
		}
	}
}

func protoV5ProviderFactories() map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"tls": providerserver.NewProtocol5WithError(New()),
	}
}

func providerVersion340() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"tls": {
			VersionConstraint: "3.4.0",
			Source:            "hashicorp/tls",
		},
	}
}

func providerVersion310() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"tls": {
			VersionConstraint: "3.1.0",
			Source:            "hashicorp/tls",
		},
	}
}

func TestProvider_InvalidProxyConfig(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []resource.TestStep{
			{
				Config: `
					provider "tls" {
						proxy {
							url = "https://proxy.host.com"
							from_env = true
						}
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: `
					provider "tls" {
						proxy {
							username = "user"
						}
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: `
					provider "tls" {
						proxy {
							password = "pwd"
						}
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: `
					provider "tls" {
						proxy {
							username = "user"
							password = "pwd"
						}
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: `
					provider "tls" {
						proxy {
							username = "user"
							from_env = true
						}
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}
