package provider

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"testing"
	"time"

	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/hashicorp/terraform-provider-tls/internal/provider/fixtures"
	tu "github.com/hashicorp/terraform-provider-tls/internal/provider/testutils"
)

func TestResourceSelfSignedCert(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: selfSignedCertConfig(1, 0),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_self_signed_cert.test1", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_self_signed_cert.test1", "cert_pem", &pkix.Name{
						SerialNumber:       "2",
						CommonName:         "example.com",
						Organization:       []string{"Example, Inc"},
						OrganizationalUnit: []string{"Department of Terraform Testing"},
						StreetAddress:      []string{"5879 Cotton Link"},
						Locality:           []string{"Pirate Harbor"},
						Province:           []string{"CA"},
						Country:            []string{"US"},
						PostalCode:         []string{"95559-1227"},
					}),
					tu.TestCheckPEMCertificateDNSNames("tls_self_signed_cert.test1", "cert_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_self_signed_cert.test1", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_self_signed_cert.test1", "cert_pem", []*url.URL{
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "ca",
						},
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "ca2",
						},
					}),
					tu.TestCheckPEMCertificateKeyUsage("tls_self_signed_cert.test1", "cert_pem", x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_self_signed_cert.test1", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateDuration("tls_self_signed_cert.test1", "cert_pem", time.Hour),
				),
			},
			{
				Config: fmt.Sprintf(`
                   resource "tls_self_signed_cert" "test2" {
                       subject {
                           serial_number = "42"
                       }
                       validity_period_hours = 1
                       allowed_uses = []
                       private_key_pem = <<EOT
%s
EOT
                   }`, fixtures.TestPrivateKeyPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_self_signed_cert.test2", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_self_signed_cert.test2", "cert_pem", &pkix.Name{
						SerialNumber: "42",
					}),
					tu.TestCheckPEMCertificateDNSNames("tls_self_signed_cert.test2", "cert_pem", []string{}),
					tu.TestCheckPEMCertificateIPAddresses("tls_self_signed_cert.test2", "cert_pem", []net.IP{}),
					tu.TestCheckPEMCertificateURIs("tls_self_signed_cert.test2", "cert_pem", []*url.URL{}),
					tu.TestCheckPEMCertificateKeyUsage("tls_self_signed_cert.test2", "cert_pem", x509.KeyUsage(0)),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_self_signed_cert.test2", "cert_pem", []x509.ExtKeyUsage{}),
				),
			},
		},
	})
}

func TestAccResourceSelfSignedCert_UpgradeFromVersion3_4_0(t *testing.T) {
	r.Test(t, r.TestCase{
		Steps: []r.TestStep{
			{
				ExternalProviders: providerVersion340(),
				Config:            selfSignedCertConfig(1, 0),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_self_signed_cert.test1", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_self_signed_cert.test1", "cert_pem", &pkix.Name{
						SerialNumber:       "2",
						CommonName:         "example.com",
						Organization:       []string{"Example, Inc"},
						OrganizationalUnit: []string{"Department of Terraform Testing"},
						StreetAddress:      []string{"5879 Cotton Link"},
						Locality:           []string{"Pirate Harbor"},
						Province:           []string{"CA"},
						Country:            []string{"US"},
						PostalCode:         []string{"95559-1227"},
					}),
					tu.TestCheckPEMCertificateDNSNames("tls_self_signed_cert.test1", "cert_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_self_signed_cert.test1", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_self_signed_cert.test1", "cert_pem", []*url.URL{
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "ca",
						},
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "ca2",
						},
					}),
					tu.TestCheckPEMCertificateKeyUsage("tls_self_signed_cert.test1", "cert_pem", x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_self_signed_cert.test1", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateDuration("tls_self_signed_cert.test1", "cert_pem", time.Hour),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   selfSignedCertConfig(1, 0),
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   selfSignedCertConfig(1, 0),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_self_signed_cert.test1", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_self_signed_cert.test1", "cert_pem", &pkix.Name{
						SerialNumber:       "2",
						CommonName:         "example.com",
						Organization:       []string{"Example, Inc"},
						OrganizationalUnit: []string{"Department of Terraform Testing"},
						StreetAddress:      []string{"5879 Cotton Link"},
						Locality:           []string{"Pirate Harbor"},
						Province:           []string{"CA"},
						Country:            []string{"US"},
						PostalCode:         []string{"95559-1227"},
					}),
					tu.TestCheckPEMCertificateDNSNames("tls_self_signed_cert.test1", "cert_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_self_signed_cert.test1", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_self_signed_cert.test1", "cert_pem", []*url.URL{
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "ca",
						},
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "ca2",
						},
					}),
					tu.TestCheckPEMCertificateKeyUsage("tls_self_signed_cert.test1", "cert_pem", x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_self_signed_cert.test1", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateDuration("tls_self_signed_cert.test1", "cert_pem", time.Hour),
				),
			},
		},
	})
}

func TestResourceSelfSignedCert_DetectExpiringAndExpired(t *testing.T) {
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: selfSignedCertConfig(10, 2),
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T21:30:00Z"),
				Config:             selfSignedCertConfig(10, 2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T23:30:00Z"),
				Config:             selfSignedCertConfig(10, 2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_DetectExpiring_Refresh(t *testing.T) {
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: selfSignedCertConfig(10, 2),
				Check:  r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "false"),
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T21:30:00Z"),
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "true"),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T21:30:00Z"),
				Config:    selfSignedCertConfig(10, 2),
				Check:     r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "false"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_DetectExpired_Refresh(t *testing.T) {
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: selfSignedCertConfig(10, 2),
				Check:  r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "false"),
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T23:30:00Z"),
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "true"),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T23:30:00Z"),
				Config:    selfSignedCertConfig(10, 2),
				Check:     r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "false"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_ReadyForRenewal_ValidityPeriodZero(t *testing.T) {
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config:             selfSignedCertConfig(0, 0),
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "true"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_ReadyForRenewal_EarlyRenewalGreaterThanValidityPeriod(t *testing.T) {
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config:             selfSignedCertConfig(1, 2),
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "true"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_ReadyForRenewal_EarlyRenewalEqualsValidityPeriod(t *testing.T) {
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config:             selfSignedCertConfig(1, 1),
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_self_signed_cert.test1", "ready_for_renewal", "true"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_RecreatesAfterExpired(t *testing.T) {
	oldNow := overridableTimeFunc
	var previousCert string
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: selfSignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					previousCert = value
					return nil
				}),
			},
			{
				Config: selfSignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					if previousCert != value {
						return fmt.Errorf("certificate updated even though no time has passed")
					}

					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T19:00:00Z"),
				Config:    selfSignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					if previousCert != value {
						return fmt.Errorf("certificate updated even though not enough time has passed")
					}

					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T21:00:00Z"),
				Config:    selfSignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					if previousCert == value {
						return fmt.Errorf("certificate not updated even though passed early renewal")
					}

					previousCert = value
					return nil
				}),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_NotRecreatedForEarlyRenewalUpdateInFuture(t *testing.T) {
	oldNow := overridableTimeFunc
	var previousCert string
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: selfSignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					previousCert = value
					return nil
				}),
			},
			{
				Config: selfSignedCertConfig(10, 3),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					if previousCert != value {
						return fmt.Errorf("certificate updated even though still time until early renewal")
					}

					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T16:00:00Z"),
				Config:    selfSignedCertConfig(10, 3),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					if previousCert != value {
						return fmt.Errorf("certificate updated even though still time until early renewal")
					}

					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T16:00:00Z"),
				Config:    selfSignedCertConfig(10, 9),
				Check: r.TestCheckResourceAttrWith("tls_self_signed_cert.test1", "cert_pem", func(value string) error {
					if previousCert == value {
						return fmt.Errorf("certificate not updated even though early renewal time has passed")
					}

					previousCert = value
					return nil
				}),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceSelfSignedCert_KeyIDs(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "tls_self_signed_cert" "test" {
						subject {
							serial_number = "42"
						}
						validity_period_hours = 1
						allowed_uses = []
						set_subject_key_id = true
						private_key_pem = <<EOT
%s
EOT
					}
				`, fixtures.TestPrivateKeyPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMCertificateSubjectKeyID("tls_self_signed_cert.test", "cert_pem", fixtures.TestPrivateKeyPEMSubjectKeyID),
					tu.TestCheckPEMCertificateNoAuthorityKeyID("tls_self_signed_cert.test", "cert_pem"),
				),
			},
			{
				Config: fmt.Sprintf(`
					resource "tls_self_signed_cert" "test" {
						subject {
							serial_number = "42"
						}
						validity_period_hours = 1
						allowed_uses = []
						set_subject_key_id = true
						set_authority_key_id = true
						private_key_pem = <<EOT
%s
EOT
					}
				`, fixtures.TestPrivateKeyPEM),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMCertificateSubjectKeyID("tls_self_signed_cert.test", "cert_pem", fixtures.TestPrivateKeyPEMSubjectKeyID),
					tu.TestCheckPEMCertificateAuthorityKeyID("tls_self_signed_cert.test", "cert_pem", fixtures.TestPrivateKeyPEMSubjectKeyID),
				),
			},
		},
	})
}

func TestResourceSelfSignedCert_InvalidConfigs(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_self_signed_cert" "test" {
						subject {
							common_name = "common test cert"
						}
						validity_period_hours = -1
						allowed_uses = [
						]
						set_subject_key_id = true
						private_key_pem = "does not matter"
					}
				`,
				ExpectError: regexp.MustCompile(`Attribute validity_period_hours value must be at least 0, got: -1`),
			},
			{
				Config: `
					resource "tls_self_signed_cert" "test" {
						subject {
							common_name = "common test cert"
						}
						validity_period_hours = 20
						early_renewal_hours = -10
						allowed_uses = [
						]
						set_subject_key_id = true
						private_key_pem = "does not matter"
					}
				`,
				ExpectError: regexp.MustCompile(`Attribute early_renewal_hours value must be at least 0, got: -10`),
			},
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						set_subject_key_id    = true
						validity_period_hours = 8760
						subject {}
						subject {}
						allowed_uses = [
							"key_encipherment",
						]
						ip_addresses = [
							"127.0.0.2",
						]
					}
				`,
				ExpectError: regexp.MustCompile(`List must contain at least 0 elements and at most 1 elements, got: 2|No more than 1 "subject" blocks are allowed|Attribute subject list must contain at least 0 elements and at most 1\nelements, got: 2|The configuration should declare a maximum of 1 block, however 2 blocks were\nconfigured`),
			},
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						is_ca_certificate     = true
						set_subject_key_id    = true
						validity_period_hours = 8760
						allowed_uses = [
							"key_encipherment",
						]
						ip_addresses = [
							"127.0.0.2",
						]
					}
				`,
				ExpectError: regexp.MustCompile(`Must contain at least one Distinguished Name`),
			},
			{
				Config: fmt.Sprintf(`
					resource "tls_self_signed_cert" "test" {
						subject {
							serial_number = "42"
						}
						validity_period_hours = 1
						allowed_uses = []
						set_authority_key_id = true
						private_key_pem = <<EOT
%s
EOT
					}
				`, fixtures.TestPrivateKeyPEM),
				ExpectError: regexp.MustCompile("Could not determine Authority Key Identifier"),
			},
		},
	})
}

func selfSignedCertConfig(validity, earlyRenewal uint32) string {
	return fmt.Sprintf(`
        resource "tls_self_signed_cert" "test1" {
            subject {
                common_name = "example.com"
                organization = "Example, Inc"
                organizational_unit = "Department of Terraform Testing"
                street_address = ["5879 Cotton Link"]
                locality = "Pirate Harbor"
                province = "CA"
                country = "US"
                postal_code = "95559-1227"
                serial_number = "2"
            }

            dns_names = [
                "example.com",
                "example.net",
            ]

            ip_addresses = [
                "127.0.0.1",
                "127.0.0.2",
            ]

            uris = [
                "spiffe://example-trust-domain/ca",
                "spiffe://example-trust-domain/ca2",
            ]

            validity_period_hours = %d
            early_renewal_hours = %d

            allowed_uses = [
                "key_encipherment",
                "digital_signature",
                "server_auth",
                "client_auth",
                "content_commitment",
            ]

            private_key_pem = <<EOT
%s
EOT
        }`, validity, earlyRenewal, fixtures.TestPrivateKeyPEM)
}

func TestResourceSelfSignedCert_FromED25519PrivateKeyResource(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_self_signed_cert.test", "key_algorithm", "ED25519"),
					tu.TestCheckPEMFormat("tls_self_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceSelfSignedCert_FromECDSAPrivateKeyResource(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm   = "ECDSA"
						ecdsa_curve = "P521"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						set_subject_key_id    = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_self_signed_cert.test", "key_algorithm", "ECDSA"),
					tu.TestCheckPEMFormat("tls_self_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceSelfSignedCert_FromECDSAPrivateKeyResource_PKCS8(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm   = "ECDSA"
						ecdsa_curve = "P521"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem_pkcs8
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						set_subject_key_id    = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_self_signed_cert.test", "key_algorithm", "ECDSA"),
					tu.TestCheckPEMFormat("tls_self_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceSelfSignedCert_FromRSAPrivateKeyResource(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "RSA"
						rsa_bits  = 4096
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						set_subject_key_id    = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_self_signed_cert.test", "key_algorithm", "RSA"),
					tu.TestCheckPEMFormat("tls_self_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceSelfSignedCert_NoSubject(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						set_subject_key_id    = true
						validity_period_hours = 8760
						subject {}
						allowed_uses = [
							"key_encipherment",
							"digital_signature",
							"server_auth",
							"client_auth",
							"cert_signing",
						]
						dns_names = [
							"pippo.pluto.paperino",
						]
						ip_addresses = [
							"127.0.0.2",
						]
						uris = [
							"disney://pippo.pluto.paperino/minnie",
						]
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_self_signed_cert.test", "key_algorithm", "ED25519"),
					tu.TestCheckPEMFormat("tls_self_signed_cert.test", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateNoSubject("tls_self_signed_cert.test", "cert_pem"),
					tu.TestCheckPEMCertificateKeyUsage("tls_self_signed_cert.test", "cert_pem", x509.KeyUsageCertSign|x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_self_signed_cert.test", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateDNSNames("tls_self_signed_cert.test", "cert_pem", []string{
						"pippo.pluto.paperino",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_self_signed_cert.test", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_self_signed_cert.test", "cert_pem", []*url.URL{
						{
							Scheme: "disney",
							Host:   "pippo.pluto.paperino",
							Path:   "minnie",
						},
					}),
				),
			},
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						set_subject_key_id    = true
						validity_period_hours = 8760
						allowed_uses = [
							"key_encipherment",
							"digital_signature",
							"server_auth",
							"client_auth",
							"cert_signing",
						]
						dns_names = [
							"pippo.pluto.paperino",
						]
						ip_addresses = [
							"127.0.0.2",
						]
						uris = [
							"disney://pippo.pluto.paperino/minnie",
						]
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_self_signed_cert.test", "key_algorithm", "ED25519"),
					tu.TestCheckPEMFormat("tls_self_signed_cert.test", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateNoSubject("tls_self_signed_cert.test", "cert_pem"),
					tu.TestCheckPEMCertificateKeyUsage("tls_self_signed_cert.test", "cert_pem", x509.KeyUsageCertSign|x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_self_signed_cert.test", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateDNSNames("tls_self_signed_cert.test", "cert_pem", []string{
						"pippo.pluto.paperino",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_self_signed_cert.test", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_self_signed_cert.test", "cert_pem", []*url.URL{
						{
							Scheme: "disney",
							Host:   "pippo.pluto.paperino",
							Path:   "minnie",
						},
					}),
				),
			},
		},
	})
}
