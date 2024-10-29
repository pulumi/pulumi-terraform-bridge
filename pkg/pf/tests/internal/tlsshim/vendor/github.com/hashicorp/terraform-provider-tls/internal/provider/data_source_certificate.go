package provider

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-provider-tls/internal/provider/attribute_validator"
)

type certificateDataSource struct {
	provider *tlsProvider
}

var _ datasource.DataSource = (*certificateDataSource)(nil)

func NewCertificateDataSource() datasource.DataSource {
	return &certificateDataSource{}
}

func (d *certificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (d *certificateDataSource) Schema(_ context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required attributes
			"url": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					attribute_validator.UrlWithScheme(supportedURLSchemesStr()...),
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("content"),
						path.MatchRoot("url"),
					),
				},
				MarkdownDescription: "URL of the endpoint to get the certificates from. " +
					fmt.Sprintf("Accepted schemes are: `%s`. ", strings.Join(supportedURLSchemesStr(), "`, `")) +
					"For scheme `https://` it will use the HTTP protocol and apply the `proxy` configuration " +
					"of the provider, if set. For scheme `tls://` it will instead use a secure TCP socket.",
			},
			"content": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("content"),
						path.MatchRoot("url"),
					),
				},
				MarkdownDescription: "The content of the certificate in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
			},

			// Optional attributes
			"verify_chain": schema.BoolAttribute{
				Optional: true,
				Validators: []validator.Bool{
					boolvalidator.ConflictsWith(
						path.MatchRoot("content"),
					),
				},
				MarkdownDescription: "Whether to verify the certificate chain while parsing it or not (default: `true`).",
			},

			// Computed attributes
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of this data source: hashing of the certificates in the chain.",
			},
			"certificates": schema.ListAttribute{
				ElementType: types.ObjectType{
					AttrTypes: x509CertObjectAttrTypes(),
				},
				Computed:            true,
				MarkdownDescription: "The certificates protecting the site, with the root of the chain first.",
			},
		},
		MarkdownDescription: "Get information about the TLS certificates securing a host.\n\n" +
			"Use this data source to get information, such as SHA1 fingerprint or serial number, " +
			"about the TLS certificates that protects a URL.",
	}
}

func (d *certificateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.provider, resp.Diagnostics = toProvider(req.ProviderData)
}

func (ds *certificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var newState certificateDataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &newState)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Enforce `verify_chain` to `true` if not set.
	//
	// NOTE: Currently it's not possible to specify `Default` values against
	// attributes of Data Sources nor Providers.
	if newState.VerifyChain.IsNull() {
		newState.VerifyChain = types.BoolValue(true)
	}

	var certs []CertificateModel
	if !newState.Content.IsNull() && !newState.Content.IsUnknown() {
		block, _ := pem.Decode([]byte(newState.Content.ValueString()))
		if block == nil {
			res.Diagnostics.AddAttributeError(
				path.Root("content"),
				"Failed to decoded PEM",
				"Value is not a valid PEM encoding of a certificate",
			)
			return
		}

		preamble, err := pemBlockToPEMPreamble(block)
		if err != nil {
			res.Diagnostics.AddError("Failed to identify PEM preamble", err.Error())
			return
		}

		if preamble != PreambleCertificate {
			res.Diagnostics.AddError(
				"Unexpected PEM preamble",
				fmt.Sprintf("Certificate PEM should be %q, got %q", PreambleCertificate, preamble),
			)
			return
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			res.Diagnostics.AddError("Unable to parse certificate", err.Error())
			return
		}

		certs = []CertificateModel{certificateToStruct(cert)}
	} else {
		targetURL, err := url.Parse(newState.URL.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(
				path.Root("url"),
				"Failed to parse URL",
				err.Error(),
			)
			return
		}

		// Determine if we should verify the chain of certificates, or skip said verification
		shouldVerifyChain := newState.VerifyChain.ValueBool()

		// Ensure a port is set on the URL, or return an error
		var peerCerts []*x509.Certificate
		switch targetURL.Scheme {
		case HTTPSScheme.String():
			if targetURL.Port() == "" {
				targetURL.Host += ":443"
			}

			peerCerts, err = fetchPeerCertificatesViaHTTPS(ctx, targetURL, shouldVerifyChain, ds.provider)
		case TLSScheme.String():
			if targetURL.Port() == "" {
				res.Diagnostics.AddError("URL malformed", fmt.Sprintf("Port missing from URL: %s", targetURL.String()))
				return
			}

			peerCerts, err = fetchPeerCertificatesViaTLS(ctx, targetURL, shouldVerifyChain)
		default:
			// NOTE: This should never happen, given we validate this at the schema level
			res.Diagnostics.AddError("Unsupported scheme", fmt.Sprintf("Scheme %q not supported", targetURL.String()))
			return
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to identify fetch peer certificates", err.Error())
			return
		}

		// Convert peer certificates to a simple map
		certs = make([]CertificateModel, len(peerCerts))
		for i, peerCert := range peerCerts {
			certs[len(peerCerts)-i-1] = certificateToStruct(peerCert)
		}
	}

	// Set certificates on the state model
	res.Diagnostics.Append(tfsdk.ValueFrom(ctx, certs, types.ListType{
		ElemType: types.ObjectType{
			AttrTypes: x509CertObjectAttrTypes(),
		},
	}, &newState.Certificates)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Set ID as hashing of the certificates
	newState.ID = types.StringValue(hashForState(fmt.Sprintf("%v", certs)))

	// Finally, set the state
	res.Diagnostics.Append(res.State.Set(ctx, newState)...)
}

func fetchPeerCertificatesViaTLS(ctx context.Context, targetURL *url.URL, shouldVerifyChain bool) ([]*x509.Certificate, error) {
	tflog.Debug(ctx, "Fetching certificate via TLS client")

	conn, err := tls.Dial("tcp", targetURL.Host, &tls.Config{
		InsecureSkipVerify: !shouldVerifyChain,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to execute TLS connection towards %s: %w", targetURL.Host, err)
	}
	defer conn.Close()

	return conn.ConnectionState().PeerCertificates, nil
}

func fetchPeerCertificatesViaHTTPS(ctx context.Context, targetURL *url.URL, shouldVerifyChain bool, p *tlsProvider) ([]*x509.Certificate, error) {
	tflog.Debug(ctx, "Fetching certificate via HTTP(S) client")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !shouldVerifyChain,
			},
			Proxy: p.proxyForRequestFunc(ctx),
		},
	}

	// First attempting an HTTP HEAD: if it fails, ignore errors and move on
	tflog.Debug(ctx, "Attempting HTTP HEAD to fetch certificates", map[string]interface{}{
		"targetURL": targetURL.String(),
	})
	resp, err := client.Head(targetURL.String())
	if err == nil && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		defer resp.Body.Close()
		return resp.TLS.PeerCertificates, nil
	}

	// Then attempting HTTP GET: if this fails we will than report the error
	tflog.Debug(ctx, "Attempting HTTP GET to fetch certificates", map[string]interface{}{
		"targetURL": targetURL.String(),
	})
	resp, err = client.Get(targetURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificates from URL '%s': %w", targetURL.Scheme, err)
	}
	defer resp.Body.Close()
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		return resp.TLS.PeerCertificates, nil
	}

	return nil, fmt.Errorf("got back response (status: %s) with no certificates from URL '%s': %w", resp.Status, targetURL.Scheme, err)
}

func certificateToStruct(cert *x509.Certificate) CertificateModel {
	certPem := string(pem.EncodeToMemory(&pem.Block{Type: PreambleCertificate.String(), Bytes: cert.Raw}))

	return CertificateModel{
		SignatureAlgorithm: types.StringValue(cert.SignatureAlgorithm.String()),
		PublicKeyAlgorithm: types.StringValue(cert.PublicKeyAlgorithm.String()),
		SerialNumber:       types.StringValue(cert.SerialNumber.String()),
		IsCA:               types.BoolValue(cert.IsCA),
		Version:            types.Int64Value(int64(cert.Version)),
		Issuer:             types.StringValue(cert.Issuer.String()),
		Subject:            types.StringValue(cert.Subject.String()),
		NotBefore:          types.StringValue(cert.NotBefore.Format(time.RFC3339)),
		NotAfter:           types.StringValue(cert.NotAfter.Format(time.RFC3339)),
		SHA1Fingerprint:    types.StringValue(fmt.Sprintf("%x", sha1.Sum(cert.Raw))),
		CertPEM:            types.StringValue(certPem),
	}
}

func x509CertObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"signature_algorithm":  types.StringType,
		"public_key_algorithm": types.StringType,
		"serial_number":        types.StringType,
		"is_ca":                types.BoolType,
		"version":              types.Int64Type,
		"issuer":               types.StringType,
		"subject":              types.StringType,
		"not_before":           types.StringType,
		"not_after":            types.StringType,
		"sha1_fingerprint":     types.StringType,
		"cert_pem":             types.StringType,
	}
}
