package provider

import (
	"crypto/x509/pkix"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"testing"

	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	tu "github.com/hashicorp/terraform-provider-tls/internal/provider/testutils"
)

func TestResourceCertRequest(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test1" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test1" {
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
							"spiffe://example-trust-domain/workload",
							"spiffe://example-trust-domain/workload2",
						]
						private_key_pem = tls_private_key.test1.private_key_pem
					}
                `,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test1", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestSubject("tls_cert_request.test1", "cert_request_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test1", "cert_request_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test1", "cert_request_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test1", "cert_request_pem", []*url.URL{
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
				),
			},
			{
				Config: `
					resource "tls_private_key" "test2" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test2" {
						subject {
							serial_number = "42"
						}
						private_key_pem = tls_private_key.test2.private_key_pem
					}
                `,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test2", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestSubject("tls_cert_request.test2", "cert_request_pem", &pkix.Name{
						SerialNumber: "42",
					}),
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test2", "cert_request_pem", []string{}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test2", "cert_request_pem", []net.IP{}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test2", "cert_request_pem", []*url.URL{}),
				),
			},
		},
	})
}

func TestAccResourceCertRequest_UpgradeFromVersion3_4_0(t *testing.T) {
    t.Parallel()
	config := `
		resource "tls_private_key" "test1" {
			algorithm = "ED25519"
		}
		resource "tls_cert_request" "test1" {
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
				"spiffe://example-trust-domain/workload",
				"spiffe://example-trust-domain/workload2",
			]
			private_key_pem = tls_private_key.test1.private_key_pem
		}`

	r.Test(t, r.TestCase{
		Steps: []r.TestStep{
			{
				ExternalProviders: providerVersion340(),
				Config:            config,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test1", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestSubject("tls_cert_request.test1", "cert_request_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test1", "cert_request_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test1", "cert_request_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test1", "cert_request_pem", []*url.URL{
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
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   config,
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   config,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test1", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestSubject("tls_cert_request.test1", "cert_request_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test1", "cert_request_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test1", "cert_request_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test1", "cert_request_pem", []*url.URL{
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
				),
			},
		},
	})
}

func TestResourceCertRequest_NoSubject(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test" {
						dns_names = [
							"pippo.pluto.paperino",
						]
						ip_addresses = [
							"127.0.0.2",
						]
						uris = [
							"disney://pippo.pluto.paperino/minnie",
						]
						private_key_pem = tls_private_key.test.private_key_pem
                    }
                `,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestNoSubject("tls_cert_request.test", "cert_request_pem"),
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test", "cert_request_pem", []string{
						"pippo.pluto.paperino",
					}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test", "cert_request_pem", []net.IP{
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test", "cert_request_pem", []*url.URL{
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
					resource "tls_cert_request" "test" {
						subject {}
						dns_names = [
							"pippo.pluto.paperino",
						]
						ip_addresses = [
							"127.0.0.2",
						]
						uris = [
							"disney://pippo.pluto.paperino/minnie",
						]
						private_key_pem = tls_private_key.test.private_key_pem
                    }
                `,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestNoSubject("tls_cert_request.test", "cert_request_pem"),
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test", "cert_request_pem", []string{
						"pippo.pluto.paperino",
					}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test", "cert_request_pem", []net.IP{
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test", "cert_request_pem", []*url.URL{
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

func TestResourceCertRequest_InvalidConfigs(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test" {
						subject {}
						subject {}
						dns_names = [
							"pippo.pluto.paperino",
						]
						ip_addresses = [
							"127.0.0.2",
						]
						uris = [
							"disney://pippo.pluto.paperino/minnie",
						]
						private_key_pem = tls_private_key.test.private_key_pem
                    }
                `,
				ExpectError: regexp.MustCompile(`List must contain at least 0 elements and at most 1 elements, got: 2|No more than 1 "subject" blocks are allowed|Attribute subject list must contain at least 0 elements and at most 1\nelements, got: 2|The configuration should declare a maximum of 1 block, however 2 blocks were\nconfigured`),
			},
		},
	})
}

func TestResourceCertRequest_PKCS8(t *testing.T) {
    t.Parallel()
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					resource "tls_private_key" "test1" {
						algorithm = "ED25519"
					}
					resource "tls_cert_request" "test1" {
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
							"spiffe://example-trust-domain/workload",
							"spiffe://example-trust-domain/workload2",
						]
						private_key_pem = tls_private_key.test1.private_key_pem_pkcs8
					}
                `,
				Check: r.ComposeAggregateTestCheckFunc(
					tu.TestCheckPEMFormat("tls_cert_request.test1", "cert_request_pem", PreambleCertificateRequest.String()),
					tu.TestCheckPEMCertificateRequestSubject("tls_cert_request.test1", "cert_request_pem", &pkix.Name{
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
					tu.TestCheckPEMCertificateRequestDNSNames("tls_cert_request.test1", "cert_request_pem", []string{
						"example.com",
						"example.net",
					}),
					tu.TestCheckPEMCertificateRequestIPAddresses("tls_cert_request.test1", "cert_request_pem", []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("127.0.0.2"),
					}),
					tu.TestCheckPEMCertificateRequestURIs("tls_cert_request.test1", "cert_request_pem", []*url.URL{
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
				),
			},
		},
	})
}

func TestResourceCertRequest_PrivateKeyPEM(t *testing.T) {
    t.Parallel()
	var pkp1, pkp2 string
	resourceName := "tls_cert_request.client_csr"

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: tlsCertRequest,
				Check: r.ComposeAggregateTestCheckFunc(
					testExtractResourceAttr(resourceName, "private_key_pem", &pkp1),
				),
			},
			{
				Taint:  []string{"tls_private_key.this"},
				Config: tlsCertRequest,
				Check: r.ComposeAggregateTestCheckFunc(
					testExtractResourceAttr(resourceName, "private_key_pem", &pkp2),
					testCheckAttributeValuesDiffer(&pkp1, &pkp2),
				),
			},
		},
	})
}

const tlsCertRequest = `
resource "tls_private_key" "this" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "tls_cert_request" "client_csr" {
  private_key_pem = tls_private_key.this.private_key_pem

  subject {
    common_name = "this"
  }
}
`

func testExtractResourceAttr(resourceName string, attributeName string, attributeValue *string) r.TestCheckFunc {
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

func testCheckAttributeValuesDiffer(i *string, j *string) r.TestCheckFunc {
	return func(s *terraform.State) error {
		if testStringValue(i) == testStringValue(j) {
			return fmt.Errorf("attribute values are the same")
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
