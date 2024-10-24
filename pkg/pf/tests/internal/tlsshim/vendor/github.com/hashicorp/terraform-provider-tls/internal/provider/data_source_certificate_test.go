package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/hashicorp/terraform-provider-tls/internal/provider/fixtures"
	tu "github.com/hashicorp/terraform-provider-tls/internal/provider/testutils"
)

func TestDataSourceCertificate_CertificateContent(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{
				Config: `
					data "tls_certificate" "test" {
						content = file("fixtures/certificate.pem")
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "1"),

					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.serial_number", "266244246501122064554217434340898012243"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.is_ca", "false"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.version", "3"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=Root CA,O=Test Org,L=Here"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=Child Cert,O=Child Co.,L=Everywhere"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_before", "2019-11-08T09:01:36Z"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_after", "2019-11-08T19:01:36Z"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.sha1_fingerprint", "61b65624427d75b61169100836904e44364df817"),
					tu.TestCheckPEMFormat("data.tls_certificate.test", "certificates.0.cert_pem", PreambleCertificate.String()),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.cert_pem", strings.TrimSpace(fixtures.TestTlsDataSourceCertFromContent)+"\n"),
				),
			},
		},
	})
}

func TestAccDataSourceCertificate_UpgradeFromVersion3_4_0(t *testing.T) {
	r.Test(t, r.TestCase{
		Steps: []r.TestStep{
			{
				ExternalProviders: providerVersion340(),
				Config: `
					data "tls_certificate" "test" {
						content = file("fixtures/certificate.pem")
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "1"),

					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.serial_number", "266244246501122064554217434340898012243"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.is_ca", "false"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.version", "3"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=Root CA,O=Test Org,L=Here"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=Child Cert,O=Child Co.,L=Everywhere"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_before", "2019-11-08T09:01:36Z"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_after", "2019-11-08T19:01:36Z"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.sha1_fingerprint", "61b65624427d75b61169100836904e44364df817"),
					tu.TestCheckPEMFormat("data.tls_certificate.test", "certificates.0.cert_pem", PreambleCertificate.String()),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.cert_pem", strings.TrimSpace(fixtures.TestTlsDataSourceCertFromContent)+"\n"),
				),
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `
					data "tls_certificate" "test" {
						content = file("fixtures/certificate.pem")
					}
				`,
				PlanOnly: true,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config: `
					data "tls_certificate" "test" {
						content = file("fixtures/certificate.pem")
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "1"),

					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.serial_number", "266244246501122064554217434340898012243"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.is_ca", "false"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.version", "3"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=Root CA,O=Test Org,L=Here"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=Child Cert,O=Child Co.,L=Everywhere"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_before", "2019-11-08T09:01:36Z"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_after", "2019-11-08T19:01:36Z"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.sha1_fingerprint", "61b65624427d75b61169100836904e44364df817"),
					tu.TestCheckPEMFormat("data.tls_certificate.test", "certificates.0.cert_pem", PreambleCertificate.String()),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.cert_pem", strings.TrimSpace(fixtures.TestTlsDataSourceCertFromContent)+"\n"),
				),
			},
		},
	})
}

// NOTE: Yes, this test is fetching a live certificate.
// It can potentially break over time, and we will need to keep the
// data we check against up to date, when that happens.
func TestAccDataSourceCertificate_TerraformIO(t *testing.T) {
	r.Test(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{
				Config: `
					data "tls_certificate" "test" {
						url = "https://www.terraform.io/"
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "3"),

					// ISRG Root X1
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=DST Root CA X3,O=Digital Signature Trust Co."),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=ISRG Root X1,O=Internet Security Research Group,C=US"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.is_ca", "true"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.sha1_fingerprint", "933c6ddee95c9c41a40f9f50493d82be03ad87bf"),

					// R3
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.issuer", "CN=ISRG Root X1,O=Internet Security Research Group,C=US"),
					r.TestCheckResourceAttrPair("data.tls_certificate.test", "certificates.1.issuer", "data.tls_certificate.test", "certificates.0.subject"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.subject", "CN=R3,O=Let's Encrypt,C=US"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.is_ca", "true"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.sha1_fingerprint", "a053375bfe84e8b748782c7cee15827a6af5a405"),

					// terraform.io
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.issuer", "CN=R3,O=Let's Encrypt,C=US"),
					r.TestCheckResourceAttrPair("data.tls_certificate.test", "certificates.2.issuer", "data.tls_certificate.test", "certificates.1.subject"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.subject", "CN=www.terraform.io"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.is_ca", "false"),
					// NOTE: Not checking the fingerprint, as this certificate is auto-updated frequently:
					//   all the other data are stable, but the signature changes every time.
				),
			},
		},
	})
}

// NOTE: Yes, this test is fetching a live certificate.
// It can potentially break over time, and we will need to keep the
// data we check against up to date, when that happens.
func TestAccDataSourceCertificate_BadSSL(t *testing.T) {
	r.Test(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{
				Config: `
					data "tls_certificate" "test" {
						url = "https://untrusted-root.badssl.com/"
					}
				`,
				ExpectError: regexp.MustCompile(`(certificate is not trusted|certificate signed by[\s]*unknown[\s]*authority)`),
			},
			{
				Config: `
					data "tls_certificate" "test" {
						url = "https://untrusted-root.badssl.com/"
						verify_chain = false
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "2"),

					// BadSSL Untrusted Root Certificate Authority
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=BadSSL Untrusted Root Certificate Authority,O=BadSSL,L=San Francisco,ST=California,C=US"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=BadSSL Untrusted Root Certificate Authority,O=BadSSL,L=San Francisco,ST=California,C=US"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.is_ca", "true"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.sha1_fingerprint", "7890c8934d5869b25d2f8d0d646f9a5d7385ba85"),

					// BadSSL
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.issuer", "CN=BadSSL Untrusted Root Certificate Authority,O=BadSSL,L=San Francisco,ST=California,C=US"),
					r.TestCheckResourceAttrPair("data.tls_certificate.test", "certificates.1.issuer", "data.tls_certificate.test", "certificates.0.subject"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.subject", "CN=*.badssl.com,O=BadSSL,L=San Francisco,ST=California,C=US"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.signature_algorithm", "SHA256-RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.public_key_algorithm", "RSA"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.is_ca", "false"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.sha1_fingerprint", "70dcc50f515cc0eef370b672683f1873943ae4ac"),
				),
			},
		},
	})
}

func TestDataSourceCertificate_CertificateContentNegativeTests(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: `
					data "tls_certificate" "test" {
						content = "not a pem"
					}
				`,
				ExpectError: regexp.MustCompile("Failed to decoded PEM"),
			},
			{
				Config: `
					data "tls_certificate" "test" {
						content = file("fixtures/private.pem")
					}
				`,
				ExpectError: regexp.MustCompile("Unexpected PEM preamble"),
			},
			{
				Config: `
					data "tls_certificate" "test" {
						content = file("fixtures/private.pem")
						url     = "https://www.hashicorp.com"
					}
				`,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
			{
				Config: `
					data "tls_certificate" "test" {}
				`,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestDataSourceCertificate_HTTPSScheme(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: fmt.Sprintf(`
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, server.Address()),
				Check: localTestCertificateChainCheckFunc(),
			},
		},
	})
}

func TestDataSourceCertificate_TLSScheme(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: fmt.Sprintf(`
					data "tls_certificate" "test" {
						url = "tls://%s"
						verify_chain = false
					}
				`, server.Address()),
				Check: localTestCertificateChainCheckFunc(),
			},
		},
	})
}

func TestDataSourceCertificate_HTTPSSchemeViaProxy(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	proxy, err := tu.NewHTTPProxyServer()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go proxy.Serve()

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							url = "http://%s"
						}
					}
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, proxy.Address(), server.Address()),
				Check: localTestCertificateChainCheckFunc(),
			},
		},
	})
}

func TestDataSourceCertificate_HTTPSSchemeViaProxyWithUsernameAuth(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	proxyUsername := "proxyUser"
	proxy, err := tu.NewHTTPProxyServerWithBasicAuth(proxyUsername, "")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go proxy.Serve()

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							url = "http://%s"
							username = "%s"
						}
					}
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, proxy.Address(), proxyUsername, server.Address()),
				Check: r.ComposeAggregateTestCheckFunc(
					localTestCertificateChainCheckFunc(),
					tu.TestCheckBothServerAndProxyWereUsed(server, proxy),
				),
			},
			{

				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							url = "http://%s"
							username = "wrong-username"
						}
					}
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, proxy.Address(), server.Address()),
				ExpectError: regexp.MustCompile("Authentication Required"),
			},
		},
	})
}

func TestDataSourceCertificate_HTTPSSchemeViaProxyWithUsernameAndPasswordAuth(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	proxyUsername := "proxyUser"
	proxyPassword := "proxyPwd"
	proxy, err := tu.NewHTTPProxyServerWithBasicAuth(proxyUsername, proxyPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go proxy.Serve()

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							url = "http://%s"
							username = "%s"
							password = "%s"
						}
					}
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, proxy.Address(), proxyUsername, proxyPassword, server.Address()),
				Check: r.ComposeAggregateTestCheckFunc(
					localTestCertificateChainCheckFunc(),
					tu.TestCheckBothServerAndProxyWereUsed(server, proxy),
				),
			},
			{

				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							url = "http://%s"
							username = "%s"
							password = "wrong-password"
						}
					}
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, proxy.Address(), proxyUsername, server.Address()),
				ExpectError: regexp.MustCompile("Authentication Required"),
			},
		},
	})
}

func TestDataSourceCertificate_HTTPSSchemeViaProxyFromEnv(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	proxy, err := tu.NewHTTPProxyServer()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go proxy.Serve()
	t.Setenv("HTTP_PROXY", fmt.Sprintf("http://%s", proxy.Address()))
	t.Setenv("HTTPS_PROXY", fmt.Sprintf("http://%s", proxy.Address()))

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							from_env = true
						}
					}
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, server.Address()),
				Check: r.ComposeAggregateTestCheckFunc(
					localTestCertificateChainCheckFunc(),
					tu.TestCheckBothServerAndProxyWereUsed(server, proxy),
				),
			},
			{
				Config: fmt.Sprintf(`
					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, server.Address()),
				Check: r.ComposeAggregateTestCheckFunc(
					localTestCertificateChainCheckFunc(),
					tu.TestCheckBothServerAndProxyWereUsed(server, proxy),
				),
			},
		},
	})
}

func TestDataSourceCertificate_HTTPSSchemeViaProxyButNoProxyAvailable(t *testing.T) {
	server, err := tu.NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go server.ServeTLS()

	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: fmt.Sprintf(`
					provider "tls" {
						proxy {
							url = "http://localhost:65535"
						}
					}

					data "tls_certificate" "test" {
						url = "https://%s"
						verify_chain = false
					}
				`, server.Address()),
				ExpectError: regexp.MustCompile(`failed to fetch certificates from URL 'https': Get "https://\[::\]:\d+":(.|\s)*proxyconnect tcp: dial tcp \[::1\]:65535`),
			},
		},
	})
}

func localTestCertificateChainCheckFunc() r.TestCheckFunc {
	return r.ComposeAggregateTestCheckFunc(
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "2"),

		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.signature_algorithm", "SHA256-RSA"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.public_key_algorithm", "RSA"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.serial_number", "60512478256160404377639062250777657301"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.is_ca", "true"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.version", "3"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=Root CA,O=Test Org,L=Here"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=Root CA,O=Test Org,L=Here"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_before", "2019-11-07T15:47:48Z"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.not_after", "2019-12-17T15:47:48Z"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.sha1_fingerprint", "5829a9bcc57f317719c5c98d1f48d6c9957cb44e"),
		tu.TestCheckPEMFormat("data.tls_certificate.test", "certificates.0.cert_pem", PreambleCertificate.String()),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.cert_pem", strings.TrimSpace(fixtures.TestTlsDataSourceCertFromURL00)+"\n"),

		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.signature_algorithm", "SHA256-RSA"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.public_key_algorithm", "RSA"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.serial_number", "266244246501122064554217434340898012243"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.is_ca", "false"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.version", "3"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.issuer", "CN=Root CA,O=Test Org,L=Here"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.subject", "CN=Child Cert,O=Child Co.,L=Everywhere"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.not_before", "2019-11-08T09:01:36Z"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.not_after", "2019-11-08T19:01:36Z"),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.sha1_fingerprint", "61b65624427d75b61169100836904e44364df817"),
		tu.TestCheckPEMFormat("data.tls_certificate.test", "certificates.1.cert_pem", PreambleCertificate.String()),
		r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.cert_pem", strings.TrimSpace(fixtures.TestTlsDataSourceCertFromURL01)+"\n"),
	)
}

func TestDataSourceCertificate_MalformedURL(t *testing.T) {
	r.UnitTest(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: `
					data "tls_certificate" "test" {
						url = "http://no.https.scheme.com"
						verify_chain = false
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid URL scheme`),
			},
			{

				Config: `
					data "tls_certificate" "test" {
						url = "unknown://unknown.scheme.com"
						verify_chain = false
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid URL scheme`),
			},
			{

				Config: `
					data "tls_certificate" "test" {
						url = "tls://host.without.port.com"
						verify_chain = false
					}
				`,
				ExpectError: regexp.MustCompile(`Port missing from URL: tls://host.without.port.com`),
			},
			{

				Config: `
					data "tls_certificate" "test" {
						url = "ftp://ftp.scheme.com"
						verify_chain = false
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid URL scheme`),
			},
			{

				Config: `
					data "tls_certificate" "test" {
						url = "1.2.3.4"
						verify_chain = false
					}
				`,
				ExpectError: regexp.MustCompile(`URL "1.2.3.4" contains no host`),
			},
			{

				Config: `
					data "tls_certificate" "test" {
						url = "not-a-url-at-all"
						verify_chain = false
					}
				`,
				ExpectError: regexp.MustCompile(`URL "not-a-url-at-all" contains no host`),
			},
		},
	})
}

func TestDataSourceCertificate_UnknownComputedCertificatesUntilApplied(t *testing.T) {
	r.Test(t, r.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),

		Steps: []r.TestStep{
			{

				Config: `
					resource "tls_private_key" "test" {
						algorithm = "ED25519"
					}

					data "tls_certificate" "test" {
						# This attribute value must be unknown to trigger
						# the behavior, therefore this replaces the unknown
						# value with a known value on apply, so the URL is valid.
						url = replace(tls_private_key.test.id, "/^.*$/","https://terraform.io")
					}

					output "test" {
						# This test must reference an underlying value of the
						# certificates attribute to trigger the behavior.
						value = data.tls_certificate.test.certificates[0].sha1_fingerprint
					}
				`,
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.#", "3"),

					// ISRG Root X1
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.issuer", "CN=DST Root CA X3,O=Digital Signature Trust Co."),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.0.subject", "CN=ISRG Root X1,O=Internet Security Research Group,C=US"),

					// R3
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.issuer", "CN=ISRG Root X1,O=Internet Security Research Group,C=US"),
					r.TestCheckResourceAttrPair("data.tls_certificate.test", "certificates.1.issuer", "data.tls_certificate.test", "certificates.0.subject"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.1.subject", "CN=R3,O=Let's Encrypt,C=US"),

					// terraform.io
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.issuer", "CN=R3,O=Let's Encrypt,C=US"),
					r.TestCheckResourceAttrPair("data.tls_certificate.test", "certificates.2.issuer", "data.tls_certificate.test", "certificates.1.subject"),
					r.TestCheckResourceAttr("data.tls_certificate.test", "certificates.2.subject", "CN=www.terraform.io"),
				),
			},
		},
	})
}
