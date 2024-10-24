package provider

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
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

type selfSignedCertResource struct{}

var (
	_ resource.Resource               = (*selfSignedCertResource)(nil)
	_ resource.ResourceWithModifyPlan = (*selfSignedCertResource)(nil)
)

func NewSelfSignedCertResource() resource.Resource {
	return &selfSignedCertResource{}
}

func (r *selfSignedCertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_self_signed_cert"
}

func (r *selfSignedCertResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required attributes
			"private_key_pem": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					requireReplaceIfStateContainsPEMString(),
				},
				Sensitive: true,
				Description: "Private key in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format, " +
					"that the certificate will belong to. " +
					"This can be read from a separate file using the [`file`](https://www.terraform.io/language/functions/file) " +
					"interpolation function. " +
					"Only an irreversible secure hash of the private key will be stored in the Terraform state.",
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
			"dns_names": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Description: "List of DNS names for which a certificate is being requested (i.e. certificate subjects).",
			},
			"ip_addresses": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Description: "List of IP addresses for which a certificate is being requested (i.e. certificate subjects).",
			},
			"uris": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Description: "List of URIs for which a certificate is being requested (i.e. certificate subjects).",
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
			"is_ca_certificate": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					attribute_plan_modifier_bool.DefaultValue(types.BoolValue(false)),
					boolplanmodifier.RequiresReplace(),
				},
				Description: "Is the generated certificate representing a Certificate Authority (CA) (default: `false`).",
			},
			"set_subject_key_id": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					attribute_plan_modifier_bool.DefaultValue(types.BoolValue(false)),
					boolplanmodifier.RequiresReplace(),
				},
				Description: "Should the generated certificate include a " +
					"[subject key identifier](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.2) (default: `false`).",
			},
			"set_authority_key_id": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					attribute_plan_modifier_bool.DefaultValue(types.BoolValue(false)),
					boolplanmodifier.RequiresReplace(),
				},
				Description: "Should the generated certificate include an " +
					"[authority key identifier](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.1): " +
					"for self-signed certificates this is the same value as the " +
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
			"key_algorithm": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Name of the algorithm used when generating the private key provided in `private_key_pem`. ",
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Unique identifier for this resource: the certificate serial number.",
			},
		},
		Blocks: map[string]schema.Block{
			"subject": schema.ListNestedBlock{
				// TODO Remove the validators below, once a fix for https://github.com/hashicorp/terraform-plugin-framework/issues/421 ships
				Validators: []validator.List{
					listvalidator.SizeBetween(0, 1),
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"organization": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `O`",
						},
						"common_name": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `CN`",
						},
						"organizational_unit": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `OU`",
						},
						"street_address": schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							PlanModifiers: []planmodifier.List{
								listplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `STREET`",
						},
						"locality": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `L`",
						},
						"province": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `ST`",
						},
						"country": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `C`",
						},
						"postal_code": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `PC`",
						},
						"serial_number": schema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Description: "Distinguished name: `SERIALNUMBER`",
						},
					},
				},
				MarkdownDescription: "The subject for which a certificate is being requested. " +
					"The acceptable arguments are all optional and their naming is based upon " +
					"[Issuer Distinguished Names (RFC5280)](https://tools.ietf.org/html/rfc5280#section-4.1.2.4) section.",
			},
		},
		MarkdownDescription: "Creates a **self-signed** TLS certificate in " +
			"[PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
	}
}

func (r *selfSignedCertResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating self signed certificate resource")

	// Load entire configuration into the model
	var newState selfSignedCertResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &newState)...)
	if res.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Loaded self signed certificate configuration", map[string]interface{}{
		"selfSignedCertConfig": fmt.Sprintf("%+v", newState),
	})

	// Parse the Private Key PEM
	tflog.Debug(ctx, "Parsing private key PEM")
	prvKey, algorithm, err := parsePrivateKeyPEM([]byte(newState.PrivateKeyPEM.ValueString()))
	if err != nil {
		res.Diagnostics.AddError("Failed to parse private key PEM", err.Error())
		return
	}

	// Set the Algorithm of the Private Key
	tflog.Debug(ctx, "Detected key algorithm of private key", map[string]interface{}{
		"keyAlgorithm": algorithm,
	})
	newState.KeyAlgorithm = types.StringValue(algorithm.String())

	cert := x509.Certificate{BasicConstraintsValid: true}

	// Add Subject if provided
	if !newState.Subject.IsNull() && !newState.Subject.IsUnknown() && len(newState.Subject.Elements()) > 0 {
		tflog.Debug(ctx, "Adding subject on certificate", map[string]interface{}{
			"subject": newState.Subject,
		})

		subject := make([]certificateSubjectModel, 1)
		res.Diagnostics.Append(newState.Subject.ElementsAs(ctx, &subject, false)...)
		if res.Diagnostics.HasError() {
			return
		}

		cert.Subject = createSubjectDistinguishedNames(ctx, subject[0])
	}

	// Add DNS names if provided
	if !newState.DNSNames.IsNull() && !newState.DNSNames.IsUnknown() {
		tflog.Debug(ctx, "Adding DNS names on certificate", map[string]interface{}{
			"dnsNames": newState.DNSNames,
		})

		newState.DNSNames.ElementsAs(ctx, &cert.DNSNames, false)
	}

	// Add IP addresses if provided
	if !newState.IPAddresses.IsNull() && !newState.IPAddresses.IsUnknown() {
		tflog.Debug(ctx, "Adding IP addresses on certificate", map[string]interface{}{
			"ipAddresses": newState.IPAddresses,
		})

		for _, ipElem := range newState.IPAddresses.Elements() {
			ipStr := ipElem.(types.String).ValueString()
			ip := net.ParseIP(ipStr)
			if ip == nil {
				res.Diagnostics.AddError(
					"Invalid IP address",
					fmt.Sprintf("Failed to parse %#v", ipStr),
				)
				return
			}
			cert.IPAddresses = append(cert.IPAddresses, ip)
		}
	}

	// Add URIs if provided
	if !newState.URIs.IsNull() && !newState.URIs.IsUnknown() {
		tflog.Debug(ctx, "Adding URIs on certificate", map[string]interface{}{
			"URIs": newState.URIs,
		})

		for _, uriElem := range newState.URIs.Elements() {
			uriStr := uriElem.(types.String).ValueString()
			uri, err := url.Parse(uriStr)
			if err != nil {
				res.Diagnostics.AddError(
					"Invalid URI",
					fmt.Sprintf("Failed to parse %#v: %v", uriStr, err.Error()),
				)
				return
			}
			cert.URIs = append(cert.URIs, uri)
		}
	}

	pubKey, err := privateKeyToPublicKey(prvKey)
	if err != nil {
		res.Diagnostics.AddError("Failed to get public key from private key", err.Error())
		return
	}

	certificate, diags := createCertificate(ctx, &cert, &cert, pubKey, prvKey, &req.Plan)
	if diags.HasError() {
		res.Diagnostics.Append(diags...)
		return
	}

	// Store the certificate into the state
	tflog.Debug(ctx, "Storing self signed certificate into the state")
	newState.ID = types.StringValue(certificate.id)
	newState.CertPEM = types.StringValue(certificate.certPem)
	newState.ValidityStartTime = types.StringValue(certificate.validityStartTime)
	newState.ValidityEndTime = types.StringValue(certificate.validityEndTime)
	res.Diagnostics.Append(res.State.Set(ctx, newState)...)
}

func (r *selfSignedCertResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading self signed certificate from state")

	modifyStateIfCertificateReadyForRenewal(ctx, req, res)
}

func (r *selfSignedCertResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Debug(ctx, "Updating self signed certificate")

	updatedUsingPlan(ctx, &req, res, &selfSignedCertResourceModel{})
}

func (r *selfSignedCertResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// NO-OP: Returning no error is enough for the framework to remove the resource from state.
	tflog.Debug(ctx, "Removing self signed certificate from state")
}

func (r *selfSignedCertResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	modifyPlanIfCertificateReadyForRenewal(ctx, &req, res)
}
