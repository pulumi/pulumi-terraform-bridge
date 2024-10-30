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

func TestResourceLocallySignedCert(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: locallySignedCertConfig(1, 0),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_locally_signed_cert.test", "cert_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateDNSNames("tls_locally_signed_cert.test", "cert_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_locally_signed_cert.test", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_locally_signed_cert.test", "cert_pem", []*url.URL{
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "workload",
						},
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "workload2",
						},
					}),
					tu.TestCheckPEMCertificateKeyUsage("tls_locally_signed_cert.test", "cert_pem", x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_locally_signed_cert.test", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateAgainstPEMRootCA("tls_locally_signed_cert.test", "cert_pem", []byte(fixtures.TestCACert)),
					tu.TestCheckPEMCertificateDuration("tls_locally_signed_cert.test", "cert_pem", time.Hour),
					tu.TestCheckPEMCertificateAuthorityKeyID("tls_locally_signed_cert.test", "cert_pem", fixtures.TestCAPrivateKeySubjectKeyID),
				),
			},
		},
	})
}

func TestAccResourceLocallySignedCert_UpgradeFromVersion3_4_0(t *testing.T) {
    t.Parallel()
	r.Test(t, r.TestCase{
		Steps: []r.TestStep{
			{
				ExternalProviders: providerVersion340(),
				Config:            locallySignedCertConfig(1, 0),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_locally_signed_cert.test", "cert_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateDNSNames("tls_locally_signed_cert.test", "cert_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_locally_signed_cert.test", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_locally_signed_cert.test", "cert_pem", []*url.URL{
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "workload",
						},
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "workload2",
						},
					}),
					tu.TestCheckPEMCertificateKeyUsage("tls_locally_signed_cert.test", "cert_pem", x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_locally_signed_cert.test", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateAgainstPEMRootCA("tls_locally_signed_cert.test", "cert_pem", []byte(fixtures.TestCACert)),
					tu.TestCheckPEMCertificateDuration("tls_locally_signed_cert.test", "cert_pem", time.Hour),
					tu.TestCheckPEMCertificateAuthorityKeyID("tls_locally_signed_cert.test", "cert_pem", fixtures.TestCAPrivateKeySubjectKeyID),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   locallySignedCertConfig(1, 0),
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   locallySignedCertConfig(1, 0),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
					tu.TestCheckPEMCertificateSubject("tls_locally_signed_cert.test", "cert_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateDNSNames("tls_locally_signed_cert.test", "cert_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateIPAddresses("tls_locally_signed_cert.test", "cert_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateURIs("tls_locally_signed_cert.test", "cert_pem", []*url.URL{
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "workload",
						},
						{
							Scheme: "spiffe",
							Host:   "example-trust-domain",
							Path:   "workload2",
						},
					}),
					tu.TestCheckPEMCertificateKeyUsage("tls_locally_signed_cert.test", "cert_pem", x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature),
					tu.TestCheckPEMCertificateExtKeyUsages("tls_locally_signed_cert.test", "cert_pem", []x509.ExtKeyUsage{
						x509.ExtKeyUsageServerAuth,
						x509.ExtKeyUsageClientAuth,
					}),
					tu.TestCheckPEMCertificateAgainstPEMRootCA("tls_locally_signed_cert.test", "cert_pem", []byte(fixtures.TestCACert)),
					tu.TestCheckPEMCertificateDuration("tls_locally_signed_cert.test", "cert_pem", time.Hour),
					tu.TestCheckPEMCertificateAuthorityKeyID("tls_locally_signed_cert.test", "cert_pem", fixtures.TestCAPrivateKeySubjectKeyID),
				),
			},
		},
	})
}

func TestResourceLocallySignedCert_DetectExpiringAndExpired(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: locallySignedCertConfig(10, 2),
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T21:30:00Z"),
				Config:             locallySignedCertConfig(10, 2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T23:30:00Z"),
				Config:             locallySignedCertConfig(10, 2),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceLocallySignedCert_DetectExpiring_Refresh(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: locallySignedCertConfig(10, 2),
				Check:  r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "false"),
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T21:30:00Z"),
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "true"),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T21:30:00Z"),
				Config:    locallySignedCertConfig(10, 2),
				Check:     r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "false"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceLocallySignedCert_DetectExpired_Refresh(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: locallySignedCertConfig(10, 2),
				Check:  r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "false"),
			},
			{
				PreConfig:          setTimeForTest("2019-06-14T23:30:00Z"),
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "true"),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T23:30:00Z"),
				Config:    locallySignedCertConfig(10, 2),
				Check:     r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "false"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceLocallySignedCert_ReadyForRenewal_ValidityPeriodZero(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config:             locallySignedCertConfig(0, 0),
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "true"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceLocallySignedCert_ReadyForRenewal_EarlyRenewalGreaterThanValidityPeriod(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config:             locallySignedCertConfig(1, 2),
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "true"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceLocallySignedCert_ReadyForRenewal_EarlyRenewalEqualsValidityPeriod(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config:             locallySignedCertConfig(1, 1),
				ExpectNonEmptyPlan: true,
				Check:              r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ready_for_renewal", "true"),
			},
		},
	})
	overridableTimeFunc = oldNow
}

func TestResourceLocallySignedCert_RecreatesAfterExpired(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	var previousCert string
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: locallySignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					previousCert = value
					return nil
				}),
			},
			{
				Config: locallySignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					if value != previousCert {
						return fmt.Errorf("certificate updated even though no time has passed")
					}
					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T19:00:00Z"),
				Config:    locallySignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					if value != previousCert {
						return fmt.Errorf("certificate updated even though not enough time has passed")
					}
					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T21:00:00Z"),
				Config:    locallySignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					if value == previousCert {
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

func TestResourceLocallySignedCert_NotRecreatedForEarlyRenewalUpdateInFuture(t *testing.T) {
    t.Parallel()
	oldNow := overridableTimeFunc
	var previousCert string
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		PreCheck:                 setTimeForTest("2019-06-14T12:00:00Z"),
		Steps: []r.TestStep{
			{
				Config: locallySignedCertConfig(10, 2),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					previousCert = value
					return nil
				}),
			},
			{
				Config: locallySignedCertConfig(10, 3),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					if value != previousCert {
						return fmt.Errorf("certificate updated even though still time until early renewal")
					}
					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T16:00:00Z"),
				Config:    locallySignedCertConfig(10, 3),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					if value != previousCert {
						return fmt.Errorf("certificate updated even though still time until early renewal")
					}
					previousCert = value
					return nil
				}),
			},
			{
				PreConfig: setTimeForTest("2019-06-14T16:00:00Z"),
				Config:    locallySignedCertConfig(10, 9),
				Check: r.TestCheckResourceAttrWith("tls_locally_signed_cert.test", "cert_pem", func(value string) error {
					if value == previousCert {
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

func locallySignedCertConfig(validity, earlyRenewal uint32) string {
	return fmt.Sprintf(`
        resource "tls_locally_signed_cert" "test" {
            cert_request_pem = <<EOT
%s
EOT
            validity_period_hours = %d
            early_renewal_hours = %d
            allowed_uses = [
                "key_encipherment",
                "digital_signature",
                "server_auth",
                "client_auth",
            ]
            ca_cert_pem = <<EOT
%s
EOT
            ca_private_key_pem = <<EOT
%s
EOT
        }`, fixtures.TestCertRequest, validity, earlyRenewal, fixtures.TestCACert, fixtures.TestCAPrivateKey)
}

func TestResourceLocallySignedCert_KeyIDs(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "tls_locally_signed_cert" "test" {
						cert_request_pem = <<EOT
%s
EOT
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = ["server_auth"]
						set_subject_key_id = false
						ca_cert_pem = <<EOT
%s
EOT
						ca_private_key_pem = <<EOT
%s
EOT
                    }`, fixtures.TestCertRequest, fixtures.TestCACert, fixtures.TestCAPrivateKey,
				),
				// Even if `set_subject_key_id` is set to `false`, the certificate will still get
				// an Authority Key Identifier as it's provided by the CA
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMCertificateNoSubjectKeyID("tls_locally_signed_cert.test", "cert_pem"),
					tu.TestCheckPEMCertificateAuthorityKeyID("tls_locally_signed_cert.test", "cert_pem", fixtures.TestCAPrivateKeySubjectKeyID),
				),
			},
			{
				Config: fmt.Sprintf(`
					resource "tls_locally_signed_cert" "test" {
						cert_request_pem = <<EOT
%s
EOT
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = ["server_auth"]
						set_subject_key_id = true
						ca_cert_pem = <<EOT
%s
EOT
						ca_private_key_pem = <<EOT
%s
EOT
                    }`, fixtures.TestCertRequest, fixtures.TestCACert, fixtures.TestCAPrivateKey,
				),
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMCertificateSubjectKeyID("tls_locally_signed_cert.test", "cert_pem", fixtures.TestPrivateKeyPEMSubjectKeyID),
					tu.TestCheckPEMCertificateAuthorityKeyID("tls_locally_signed_cert.test", "cert_pem", fixtures.TestCAPrivateKeySubjectKeyID),
				),
			},
			{
				Config: `
					resource "tls_private_key" "ca_prv_test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "ca_cert_test" {
						private_key_pem = tls_private_key.ca_prv_test.private_key_pem
						validity_period_hours = 8760
						allowed_uses = ["cert_signing"]
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
					}
					resource "tls_locally_signed_cert" "test" {
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = ["server_auth", "client_auth"]
						cert_request_pem = tls_cert_request.test.cert_request_pem
						ca_cert_pem = tls_self_signed_cert.ca_cert_test.cert_pem
						ca_private_key_pem = tls_private_key.ca_prv_test.private_key_pem
					}
				`,
				// NOTE: As the CA used for this certificate is a non-CA self-signed certificate that doesn't
				// carry a Subject Key Identifier, this is reflected in the child certificate that has no
				// Authority Key Identifier
				Check: tu.TestCheckPEMCertificateNoAuthorityKeyID("tls_locally_signed_cert.test", "cert_pem"),
			},
		},
	})
}

func TestResourceLocallySignedCert_FromED25519PrivateKeyResource(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "ca_prv_test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "ca_cert_test" {
						private_key_pem = tls_private_key.ca_prv_test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						subject {
							common_name  = "test.com"
						}
					}
					resource "tls_locally_signed_cert" "test" {
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = [
							"server_auth",
							"client_auth",
						]
						cert_request_pem = tls_cert_request.test.cert_request_pem
						ca_cert_pem = tls_self_signed_cert.ca_cert_test.cert_pem
						ca_private_key_pem = tls_private_key.ca_prv_test.private_key_pem
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ca_key_algorithm", "ED25519"),
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceLocallySignedCert_FromED25519PrivateKeyResource_PKCS8(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "ca_prv_test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "ca_cert_test" {
						private_key_pem = tls_private_key.ca_prv_test.private_key_pem_pkcs8
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test" {
						private_key_pem = tls_private_key.test.private_key_pem_pkcs8
						subject {
							common_name  = "test.com"
						}
					}
					resource "tls_locally_signed_cert" "test" {
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = [
							"server_auth",
							"client_auth",
						]
						cert_request_pem = tls_cert_request.test.cert_request_pem
						ca_cert_pem = tls_self_signed_cert.ca_cert_test.cert_pem
						ca_private_key_pem = tls_private_key.ca_prv_test.private_key_pem_pkcs8
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ca_key_algorithm", "ED25519"),
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceLocallySignedCert_FromECDSAPrivateKeyResource(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "ca_prv_test" {
						algorithm = "ECDSA"
					}
					resource "tls_self_signed_cert" "ca_cert_test" {
						private_key_pem = tls_private_key.ca_prv_test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
					resource "tls_private_key" "test" {
						algorithm = "ECDSA"
					}
					resource "tls_cert_request" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						subject {
							common_name  = "test.com"
						}
					}
					resource "tls_locally_signed_cert" "test" {
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = [
							"server_auth",
							"client_auth",
						]
						cert_request_pem = tls_cert_request.test.cert_request_pem
						ca_cert_pem = tls_self_signed_cert.ca_cert_test.cert_pem
						ca_private_key_pem = tls_private_key.ca_prv_test.private_key_pem
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ca_key_algorithm", "ECDSA"),
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceLocallySignedCert_FromRSAPrivateKeyResource(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "ca_prv_test" {
						algorithm = "RSA"
					}
					resource "tls_self_signed_cert" "ca_cert_test" {
						private_key_pem = tls_private_key.ca_prv_test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						validity_period_hours = 8760
						allowed_uses = [
							"cert_signing",
						]
					}
					resource "tls_private_key" "test" {
						algorithm = "RSA"
					}
					resource "tls_cert_request" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
						subject {
							common_name  = "test.com"
						}
					}
					resource "tls_locally_signed_cert" "test" {
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = [
							"server_auth",
							"client_auth",
						]
						cert_request_pem = tls_cert_request.test.cert_request_pem
						ca_cert_pem = tls_self_signed_cert.ca_cert_test.cert_pem
						ca_private_key_pem = tls_private_key.ca_prv_test.private_key_pem
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("tls_locally_signed_cert.test", "ca_key_algorithm", "RSA"),
					tu.TestCheckPEMFormat("tls_locally_signed_cert.test", "cert_pem", PreambleCertificate.String()),
				),
			},
		},
	})
}

func TestResourceLocallySignedCert_InvalidConfigs(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "ca_prv_test" {
						algorithm = "ED25519"
					}
					resource "tls_self_signed_cert" "ca_cert_test" {
						private_key_pem = tls_private_key.ca_prv_test.private_key_pem
						subject {
							organization = "test-organization"
						}
						is_ca_certificate     = true
						validity_period_hours = 8760
						allowed_uses = ["cert_signing"]
					}
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test" {
						private_key_pem = tls_private_key.test.private_key_pem
					}
					resource "tls_locally_signed_cert" "test" {
						is_ca_certificate = true
						validity_period_hours = 1
						early_renewal_hours = 0
						allowed_uses = ["server_auth", "client_auth"]
						cert_request_pem = tls_cert_request.test.cert_request_pem
						ca_cert_pem = tls_self_signed_cert.ca_cert_test.cert_pem
						ca_private_key_pem = tls_private_key.ca_prv_test.private_key_pem
					}
				`,
				ExpectError: regexp.MustCompile(`Must contain at least one Distinguished Name`),
			},
		},
	})
}
