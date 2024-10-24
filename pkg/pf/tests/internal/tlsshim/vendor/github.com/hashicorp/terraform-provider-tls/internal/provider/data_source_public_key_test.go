package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-tls/internal/provider/fixtures"
)

const (
	configDataSourcePublicKeyViaPEM = `
data "tls_public_key" "test" {
	private_key_pem = <<EOF
	%s
	EOF
}
`
	configDataSourcePublicKeyViaOpenSSHPEM = `
data "tls_public_key" "test" {
	private_key_openssh = <<EOF
	%s
	EOF
}
`
)

func TestPublicKey_dataSource_PEM(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(configDataSourcePublicKeyViaPEM, fixtures.TestPrivateKeyPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_pem", strings.TrimSpace(fixtures.TestPublicKeyPEM)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_openssh", strings.TrimSpace(fixtures.TestPublicKeyOpenSSH)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_md5", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintMD5)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_sha256", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintSHA256)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "algorithm", "RSA"),
				),
			},
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "RSA"
					}
					data "tls_public_key" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
					}
				`,
				Check: r.TestCheckResourceAttrPair(
					"data.tls_public_key.test", "public_key_pem",
					"tls_private_key.test", "public_key_pem",
				),
			},
			{
				Config: `
					resource "tls_private_key" "ecdsaPrvKey" {
						algorithm   = "ECDSA"
						ecdsa_curve = "P384"
					}
					data "tls_public_key" "ecdsaPubKey" {
						private_key_pem = tls_private_key.ecdsaPrvKey.private_key_pem
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrPair(
						"data.tls_public_key.ecdsaPubKey", "public_key_pem",
						"tls_private_key.ecdsaPrvKey", "public_key_pem",
					),
					r.TestCheckResourceAttr("data.tls_public_key.ecdsaPubKey", "algorithm", "ECDSA"),
				),
			},
			{
				Config:      fmt.Sprintf(configDataSourcePublicKeyViaPEM, "corrupt"),
				ExpectError: regexp.MustCompile(`failed to decode PEM block: decoded bytes \d, undecoded \d`),
			},
		},
	})
}

func TestPublicKey_dataSource_PEM_UpgradeFromVersion3_4_0(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		Steps: []r.TestStep{
			{
				ExternalProviders: providerVersion340(),
				Config:            fmt.Sprintf(configDataSourcePublicKeyViaPEM, fixtures.TestPrivateKeyPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_pem", strings.TrimSpace(fixtures.TestPublicKeyPEM)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_openssh", strings.TrimSpace(fixtures.TestPublicKeyOpenSSH)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_md5", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintMD5)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_sha256", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintSHA256)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "algorithm", "RSA"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   fmt.Sprintf(configDataSourcePublicKeyViaPEM, fixtures.TestPrivateKeyPEM),
				PlanOnly:                 true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   fmt.Sprintf(configDataSourcePublicKeyViaPEM, fixtures.TestPrivateKeyPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_pem", strings.TrimSpace(fixtures.TestPublicKeyPEM)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_openssh", strings.TrimSpace(fixtures.TestPublicKeyOpenSSH)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_md5", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintMD5)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_sha256", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintSHA256)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "algorithm", "RSA"),
				),
			},
		},
	})
}

func TestPublicKey_dataSource_OpenSSHPEM(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(configDataSourcePublicKeyViaOpenSSHPEM, fixtures.TestPrivateKeyOpenSSHPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_pem", strings.TrimSpace(fixtures.TestPublicKeyPEM)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_openssh", strings.TrimSpace(fixtures.TestPublicKeyOpenSSH)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_md5", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintMD5)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_sha256", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintSHA256)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "algorithm", "RSA"),
				),
			},
			{
				Config: `
					resource "tls_private_key" "rsaPrvKey" {
						algorithm = "RSA"
					}
					data "tls_public_key" "rsaPubKey" {
						private_key_openssh = tls_private_key.rsaPrvKey.private_key_openssh
					}
				`,
				Check: r.TestCheckResourceAttrPair(
					"data.tls_public_key.rsaPubKey", "public_key_openssh",
					"tls_private_key.rsaPrvKey", "public_key_openssh",
				),
			},
			{
				Config: `
					resource "tls_private_key" "ed25519PrvKey" {
						algorithm   = "ED25519"
					}
					data "tls_public_key" "ed25519PubKey" {
						private_key_openssh = tls_private_key.ed25519PrvKey.private_key_openssh
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrPair(
						"data.tls_public_key.ed25519PubKey", "public_key_openssh",
						"tls_private_key.ed25519PrvKey", "public_key_openssh",
					),
					r.TestCheckResourceAttr("data.tls_public_key.ed25519PubKey", "algorithm", "ED25519"),
				),
			},
			{
				Config:      fmt.Sprintf(configDataSourcePublicKeyViaOpenSSHPEM, "corrupt"),
				ExpectError: regexp.MustCompile("ssh: no key found"),
			},
		},
	})
}

func TestAccPublicKey_dataSource_OpenSSHPEM_UpgradeFromVersion3_4_0(t *testing.T) {
	r.Test(t, r.TestCase{
		Steps: []r.TestStep{
			{
				ExternalProviders: providerVersion340(),
				Config:            fmt.Sprintf(configDataSourcePublicKeyViaOpenSSHPEM, fixtures.TestPrivateKeyOpenSSHPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_pem", strings.TrimSpace(fixtures.TestPublicKeyPEM)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_openssh", strings.TrimSpace(fixtures.TestPublicKeyOpenSSH)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_md5", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintMD5)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_sha256", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintSHA256)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "algorithm", "RSA"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   fmt.Sprintf(configDataSourcePublicKeyViaOpenSSHPEM, fixtures.TestPrivateKeyOpenSSHPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_pem", strings.TrimSpace(fixtures.TestPublicKeyPEM)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_openssh", strings.TrimSpace(fixtures.TestPublicKeyOpenSSH)+"\n"),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_md5", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintMD5)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "public_key_fingerprint_sha256", strings.TrimSpace(fixtures.TestPublicKeyOpenSSHFingerprintSHA256)),
					r.TestCheckResourceAttr("data.tls_public_key.test", "algorithm", "RSA"),
				),
			},
		},
	})
}

func TestPublicKey_dataSource_PKCS8PEM(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "rsaPrvKey" {
						algorithm = "RSA"
					}
					data "tls_public_key" "rsaPubKey" {
						private_key_pem = tls_private_key.rsaPrvKey.private_key_pem_pkcs8
					}
				`,
				Check: r.TestCheckResourceAttrPair(
					"data.tls_public_key.rsaPubKey", "public_key_openssh",
					"tls_private_key.rsaPrvKey", "public_key_openssh",
				),
			},
			{
				Config: `
					resource "tls_private_key" "ed25519PrvKey" {
						algorithm   = "ED25519"
					}
					data "tls_public_key" "ed25519PubKey" {
						private_key_pem = tls_private_key.ed25519PrvKey.private_key_pem_pkcs8
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrPair(
						"data.tls_public_key.ed25519PubKey", "public_key_openssh",
						"tls_private_key.ed25519PrvKey", "public_key_openssh",
					),
					r.TestCheckResourceAttr("data.tls_public_key.ed25519PubKey", "algorithm", "ED25519"),
				),
			},
		},
	})
}

func TestPublicKey_dataSource_errorCases(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					data "tls_public_key" "test" {
						private_key_pem = "does not matter"
						private_key_openssh = "does not matter"
					}
				`,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
			{
				Config: `
					data "tls_public_key" "test" {
					}
				`,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}
