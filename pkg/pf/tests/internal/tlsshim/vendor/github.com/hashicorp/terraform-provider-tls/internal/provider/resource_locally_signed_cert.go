package provider

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-provider-tls/internal/provider/attribute_plan_modifier_bool"
	"github.com/hashicorp/terraform-provider-tls/internal/provider/attribute_plan_modifier_int64"
)

type locallySignedCertResource struct{}

var (
	_ resource.Resource               = (*locallySignedCertResource)(nil)
	_ resource.ResourceWithModifyPlan = (*locallySignedCertResource)(nil)
)

func NewLocallySignedCertResource() resource.Resource {
	return &locallySignedCertResource{}
}

func (r *locallySignedCertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_locally_signed_cert"
}

func (r *locallySignedCertResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required attributes
			"ca_cert_pem": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					requireReplaceIfStateContainsPEMString(),
				},
				Description: "Certificate data of the Certificate Authority (CA) " +
					"in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
			},
			"ca_private_key_pem": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					requireReplaceIfStateContainsPEMString(),
				},
				Sensitive: true,
				Description: "Private key of the Certificate Authority (CA) used to sign the certificate, " +
					"in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
			},
			"cert_request_pem": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					requireReplaceIfStateContainsPEMString(),
				},
				Description: "Certificate request data in " +
					"[PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
			},
			"validity_period_hours": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				Description: "Number of hours, after initial issuing, that the certificate will remain valid for.",
			},
			"allowed_uses": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf(supportedKeyUsagesStr()...),
					),
				},
				Description: "List of key usages allowed for the issued certificate. " +
					"Values are defined in [RFC 5280](https://datatracker.ietf.org/doc/html/rfc5280) " +
					"and combine flags defined by both " +
					"[Key Usages](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.3) " +
					"and [Extended Key Usages](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.12). " +
					fmt.Sprintf("Accepted values: `%s`.", strings.Join(supportedKeyUsagesStr(), "`, `")),
			},

			// Optional attributes
			"is_ca_certificate": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					attribute_plan_modifier_bool.DefaultValue(types.BoolValue(false)),
				},
				Description: "Is the generated certificate representing a Certificate Authority (CA) (default: `false`).",
			},
			"early_renewal_hours": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					attribute_plan_modifier_int64.DefaultValue(types.Int64Value(0)),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				Description: "The resource will consider the certificate to have expired the given number of hours " +
					"before its actual expiry time. This can be useful to deploy an updated certificate in advance of " +
					"the expiration of the current certificate. " +
					"However, the old certificate remains valid until its true expiration time, since this resource " +
					"does not (and cannot) support certificate revocation. " +
					"Also, this advance update can only be performed should the Terraform configuration be applied " +
					"during the early renewal period. (default: `0`)",
			},
			"set_subject_key_id": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					attribute_plan_modifier_bool.DefaultValue(types.BoolValue(false)),
				},
				Description: "Should the generated certificate include a " +
					"[subject key identifier](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.2) (default: `false`).",
			},

			// Computed attributes
			"cert_pem": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Certificate data in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format. " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"ready_for_renewal": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					attribute_plan_modifier_bool.DefaultValue(types.BoolValue(false)),
					attribute_plan_modifier_bool.ReadyForRenewal(),
				},
				Description: "Is the certificate either expired (i.e. beyond the `validity_period_hours`) " +
					"or ready for an early renewal (i.e. within the `early_renewal_hours`)?",
			},
			"validity_start_time": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "The time after which the certificate is valid, " +
					"expressed as an [RFC3339](https://tools.ietf.org/html/rfc3339) timestamp.",
			},
			"validity_end_time": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "The time until which the certificate is invalid, " +
					"expressed as an [RFC3339](https://tools.ietf.org/html/rfc3339) timestamp.",
			},
			"ca_key_algorithm": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Name of the algorithm used when generating the private key provided in `ca_private_key_pem`. ",
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Unique identifier for this resource: the certificate serial number.",
			},
		},
		MarkdownDescription: "Creates a TLS certificate in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) " +
			"format using a Certificate Signing Request (CSR) and signs it with a provided " +
			"(local) Certificate Authority (CA).",
	}
}

func (r *locallySignedCertResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating locally signed certificate resource")

	// Load entire configuration into the model
	var newState locallySignedCertResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &newState)...)
	if res.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Loaded locally signed certificate configuration", map[string]interface{}{
		"locallySignedCertConfig": fmt.Sprintf("%+v", newState),
	})

	// Parse the certificate request PEM
	tflog.Debug(ctx, "Parsing certificate request PEM")
	certReq, err := parseCertificateRequest([]byte(newState.CertRequestPEM.ValueString()))
	if err != nil {
		res.Diagnostics.AddError("Failed to parse certificate request PEM", err.Error())
		return
	}

	// Parse the CA Private Key PEM
	tflog.Debug(ctx, "Parsing CA private key PEM")
	caPrvKey, algorithm, err := parsePrivateKeyPEM([]byte(newState.CAPrivateKeyPEM.ValueString()))
	if err != nil {
		res.Diagnostics.AddError("Failed to parse CA private key PEM", err.Error())
		return
	}

	// Set the Algorithm of the Private Key
	tflog.Debug(ctx, "Detected key algorithm of CA private key", map[string]interface{}{
		"caKeyAlgorithm": algorithm,
	})
	newState.CAKeyAlgorithm = types.StringValue(algorithm.String())

	// Parse the CA Certificate PEM
	tflog.Debug(ctx, "Parsing CA certificate PEM")
	caCert, err := parseCertificate([]byte(newState.CACertPEM.ValueString()))
	if err != nil {
		res.Diagnostics.AddError("Failed to parse CA certificate PEM", err.Error())
		return
	}
	if !caCert.IsCA {
		tflog.Warn(ctx, "CA certificate does not appear to be a valid Certificate Authority")
		res.Diagnostics.AddWarning(
			"Potentially Invalid Certificate Authority",
			"Certificate provided as Authority does not appear to be a valid Certificate Authority. The resulting certificate might fail certificate validation.",
		)
	}

	// Prepare a template and create the certificate
	certTemplate := x509.Certificate{
		Subject:               certReq.Subject,
		DNSNames:              certReq.DNSNames,
		IPAddresses:           certReq.IPAddresses,
		URIs:                  certReq.URIs,
		BasicConstraintsValid: true,
	}
	certificate, diags := createCertificate(ctx, &certTemplate, caCert, certReq.PublicKey, caPrvKey, &req.Plan)
	if diags.HasError() {
		res.Diagnostics.Append(diags...)
		return
	}

	// Store the certificate into the state
	tflog.Debug(ctx, "Storing locally signed certificate into the state")
	newState.ID = types.StringValue(certificate.id)
	newState.CertPEM = types.StringValue(certificate.certPem)
	newState.ValidityStartTime = types.StringValue(certificate.validityStartTime)
	newState.ValidityEndTime = types.StringValue(certificate.validityEndTime)
	res.Diagnostics.Append(res.State.Set(ctx, newState)...)
}

func (r *locallySignedCertResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading locally signed certificate from state")

	modifyStateIfCertificateReadyForRenewal(ctx, req, res)
}

func (r *locallySignedCertResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Debug(ctx, "Updating locally signed certificate")

	updatedUsingPlan(ctx, &req, res, &locallySignedCertResourceModel{})
}

func (r *locallySignedCertResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// NO-OP: Returning no error is enough for the framework to remove the resource from state.
	tflog.Debug(ctx, "Removing locally signed certificate from state")
}

func (r *locallySignedCertResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	modifyPlanIfCertificateReadyForRenewal(ctx, &req, res)
}

func parseCertificate(pemBytes []byte) (*x509.Certificate, error) {
	block, err := decodePEM(pemBytes, PreambleCertificate)
	if err != nil {
		return nil, err
	}

	certs, err := x509.ParseCertificates(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	if len(certs) < 1 {
		return nil, fmt.Errorf("no certificates found")
	}
	if len(certs) > 1 {
		return nil, fmt.Errorf("multiple certificates found in")
	}

	return certs[0], nil
}

func parseCertificateRequest(pemBytes []byte) (*x509.CertificateRequest, error) {
	block, err := decodePEM(pemBytes, PreambleCertificateRequest)
	if err != nil {
		return nil, err
	}

	certReq, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate request: %w", err)
	}

	return certReq, nil
}

func decodePEM(pemBytes []byte, pemType PEMPreamble) (*pem.Block, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM of expected type %q from bytes", pemType)
	}

	if pemType.String() != block.Type {
		return nil, fmt.Errorf("invalid PEM type - expected %q, got %q", pemType, block.Type)
	}

	return block, nil
}
