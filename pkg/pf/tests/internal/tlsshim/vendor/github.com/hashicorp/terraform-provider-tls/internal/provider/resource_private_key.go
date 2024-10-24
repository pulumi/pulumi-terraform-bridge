package provider

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/terraform-provider-tls/internal/openssh"
	"github.com/hashicorp/terraform-provider-tls/internal/provider/attribute_plan_modifier_int64"
	"github.com/hashicorp/terraform-provider-tls/internal/provider/attribute_plan_modifier_string"
)

type privateKeyResource struct{}

var (
	_ resource.Resource                 = (*privateKeyResource)(nil)
	_ resource.ResourceWithUpgradeState = (*privateKeyResource)(nil)
)

func NewPrivateKeyResource() resource.Resource {
	return &privateKeyResource{}
}

func (r *privateKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_private_key"
}

func (r *privateKeyResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			// Required attributes
			"algorithm": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(supportedAlgorithmsStr()...),
				},
				Description: "Name of the algorithm to use when generating the private key. " +
					fmt.Sprintf("Currently-supported values are: `%s`. ", strings.Join(supportedAlgorithmsStr(), "`, `")),
			},

			// Optional attributes
			"rsa_bits": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					attribute_plan_modifier_int64.DefaultValue(types.Int64Value(2048)),
				},
				MarkdownDescription: "When `algorithm` is `RSA`, the size of the generated RSA key, in bits (default: `2048`).",
			},
			"ecdsa_curve": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					attribute_plan_modifier_string.DefaultValue(types.StringValue(P224.String())),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(supportedECDSACurvesStr()...),
				},
				MarkdownDescription: "When `algorithm` is `ECDSA`, the name of the elliptic curve to use. " +
					fmt.Sprintf("Currently-supported values are: `%s`. ", strings.Join(supportedECDSACurvesStr(), "`, `")) +
					fmt.Sprintf("(default: `%s`).", P224.String()),
			},

			// Computed attributes
			"private_key_pem": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key data in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
			},
			"private_key_openssh": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key data in [OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) format.",
			},
			"private_key_pem_pkcs8": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key data in [PKCS#8 PEM (RFC 5208)](https://datatracker.ietf.org/doc/html/rfc5208) format.",
			},
			"public_key_pem": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Public key data in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format. " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"public_key_openssh": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: " The public key data in " +
					"[\"Authorized Keys\"](https://www.ssh.com/academy/ssh/authorized_keys/openssh#format-of-the-authorized-keys-file) format. " +
					"This is not populated for `ECDSA` with curve `P224`, as it is [not supported](../../docs#limitations). " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"public_key_fingerprint_md5": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The fingerprint of the public key data in OpenSSH MD5 hash format, e.g. `aa:bb:cc:...`. " +
					"Only available if the selected private key format is compatible, similarly to " +
					"`public_key_openssh` and the [ECDSA P224 limitations](../../docs#limitations).",
			},
			"public_key_fingerprint_sha256": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The fingerprint of the public key data in OpenSSH SHA256 hash format, e.g. `SHA256:...`. " +
					"Only available if the selected private key format is compatible, similarly to " +
					"`public_key_openssh` and the [ECDSA P224 limitations](../../docs#limitations).",
			},
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Unique identifier for this resource: " +
					"hexadecimal representation of the SHA1 checksum of the resource.",
			},
		},
		MarkdownDescription: "Creates a PEM (and OpenSSH) formatted private key.\n\n" +
			"Generates a secure private key and encodes it in " +
			"[PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) and " +
			"[OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) formats. " +
			"This resource is primarily intended for easily bootstrapping throwaway development environments.",
	}
}

func privateKeyResourceSchemaV1() schema.Schema {
	return schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			// Required attributes
			"algorithm": schema.StringAttribute{
				Required: true,
				Description: "Name of the algorithm to use when generating the private key. " +
					fmt.Sprintf("Currently-supported values are: `%s`. ", strings.Join(supportedAlgorithmsStr(), "`, `")),
			},

			// Optional attributes
			"rsa_bits": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `algorithm` is `RSA`, the size of the generated RSA key, in bits (default: `2048`).",
			},
			"ecdsa_curve": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "When `algorithm` is `ECDSA`, the name of the elliptic curve to use. " +
					fmt.Sprintf("Currently-supported values are: `%s`. ", strings.Join(supportedECDSACurvesStr(), "`, `")) +
					fmt.Sprintf("(default: `%s`).", P224.String()),
			},

			// Computed attributes
			"private_key_pem": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key data in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format.",
			},
			"private_key_openssh": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key data in [OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) format.",
			},
			"private_key_pem_pkcs8": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Private key data in [PKCS#8 PEM (RFC 5208)](https://datatracker.ietf.org/doc/html/rfc5208) format.",
			},
			"public_key_pem": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Public key data in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format. " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"public_key_openssh": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: " The public key data in " +
					"[\"Authorized Keys\"](https://www.ssh.com/academy/ssh/authorized_keys/openssh#format-of-the-authorized-keys-file) format. " +
					"This is not populated for `ECDSA` with curve `P224`, as it is [not supported](../../docs#limitations). " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"public_key_fingerprint_md5": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The fingerprint of the public key data in OpenSSH MD5 hash format, e.g. `aa:bb:cc:...`. " +
					"Only available if the selected private key format is compatible, similarly to " +
					"`public_key_openssh` and the [ECDSA P224 limitations](../../docs#limitations).",
			},
			"public_key_fingerprint_sha256": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The fingerprint of the public key data in OpenSSH SHA256 hash format, e.g. `SHA256:...`. " +
					"Only available if the selected private key format is compatible, similarly to " +
					"`public_key_openssh` and the [ECDSA P224 limitations](../../docs#limitations).",
			},
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Unique identifier for this resource: " +
					"hexadecimal representation of the SHA1 checksum of the resource.",
			},
		},
		MarkdownDescription: "Creates a PEM (and OpenSSH) formatted private key.\n\n" +
			"Generates a secure private key and encodes it in " +
			"[PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) and " +
			"[OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) formats. " +
			"This resource is primarily intended for easily bootstrapping throwaway development environments.",
	}
}

func (r *privateKeyResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating private key resource")

	// Load entire configuration into the model
	var newState privateKeyResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &newState)...)
	if res.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Loaded private key configuration", map[string]interface{}{
		"privateKeyConfig": fmt.Sprintf("%+v", newState),
	})

	keyAlgoName := Algorithm(newState.Algorithm.ValueString())

	// Identify the correct (Private) Key Generator
	var keyGen keyGenerator
	var ok bool
	if keyGen, ok = keyGenerators[keyAlgoName]; !ok {
		res.Diagnostics.AddError("Invalid Key Algorithm", fmt.Sprintf("Key Algorithm %q is not supported", keyAlgoName))
		return
	}

	// Generate the new Key
	tflog.Debug(ctx, "Generating private key for algorithm", map[string]interface{}{
		"algorithm": keyAlgoName,
	})
	prvKey, err := keyGen(&newState)
	if err != nil {
		res.Diagnostics.AddError("Unable to generate Key from configuration", err.Error())
		return
	}

	// Marshal the Key in PEM block
	tflog.Debug(ctx, "Marshalling private key to PEM")
	var prvKeyPemBlock *pem.Block
	switch k := prvKey.(type) {
	case *rsa.PrivateKey:
		prvKeyPemBlock = &pem.Block{
			Type:  PreamblePrivateKeyRSA.String(),
			Bytes: x509.MarshalPKCS1PrivateKey(k),
		}
	case *ecdsa.PrivateKey:
		keyBytes, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			res.Diagnostics.AddError("Unable to encode key to PEM", err.Error())
			return
		}

		prvKeyPemBlock = &pem.Block{
			Type:  PreamblePrivateKeyEC.String(),
			Bytes: keyBytes,
		}
	case ed25519.PrivateKey:
		prvKeyBytes, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			res.Diagnostics.AddError("Unable to encode key to PEM", err.Error())
			return
		}

		prvKeyPemBlock = &pem.Block{
			Type:  PreamblePrivateKeyPKCS8.String(),
			Bytes: prvKeyBytes,
		}
	default:
		res.Diagnostics.AddError("Unsupported private key type", fmt.Sprintf("Key type %T not supported", prvKey))
		return
	}

	// Marshal the Key in PKCS#8 PEM block
	tflog.Debug(ctx, "Marshalling private key to PKCS#8 PEM")
	prvKeyPKCS8PemBlock, err := prvKeyToPKCS8PEMBlock(prvKey)
	if err != nil {
		res.Diagnostics.AddError("Unable to encode private key to PKCS#8 PEM", err.Error())
		return
	}

	newState.PrivateKeyPem = types.StringValue(string(pem.EncodeToMemory(prvKeyPemBlock)))
	newState.PrivateKeyPKCS8 = types.StringValue(string(pem.EncodeToMemory(prvKeyPKCS8PemBlock)))

	// Marshal the Key in OpenSSH PEM block, if supported
	tflog.Debug(ctx, "Marshalling private key to OpenSSH PEM (if supported)")
	newState.PrivateKeyOpenSSH = types.StringValue("")
	if prvKeySupportsOpenSSHMarshalling(prvKey) {
		openSSHKeyPemBlock, err := openssh.MarshalPrivateKey(prvKey, "")
		if err != nil {
			res.Diagnostics.AddError("Unable to marshal private key into OpenSSH format", err.Error())
			return
		}

		newState.PrivateKeyOpenSSH = types.StringValue(string(pem.EncodeToMemory(openSSHKeyPemBlock)))
	}

	// Store the model populated so far, onto the State
	tflog.Debug(ctx, "Storing private key info into the state")
	res.Diagnostics.Append(res.State.Set(ctx, newState)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Store the rest of the "public key" attributes onto the State
	tflog.Debug(ctx, "Storing private key's public key info into the state")
	res.Diagnostics.Append(setPublicKeyAttributes(ctx, &res.State, prvKey)...)
}

func (r *privateKeyResource) Read(ctx context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// NO-OP: all there is to read is in the State, and response is already populated with that.
	tflog.Debug(ctx, "Reading private key from state")
}

func (r *privateKeyResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// NO-OP: changes to this resource will force a "re-create".
}

func (r *privateKeyResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// NO-OP: Returning no error is enough for the framework to remove the resource from state.
	tflog.Debug(ctx, "Removing private key from state")
}

func (r *privateKeyResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	schemaV1 := privateKeyResourceSchemaV1()

	return map[int64]resource.StateUpgrader{
		// Upgrading schema v0 -> v1 will add:
		// * `private_key_openssh` (introduced in v3.2.0)
		// * `public_key_fingerprint_sha256` (introduced in v3.2.0)
		// * `private_key_pem_pkcs8`   (introduced in v4.0.0)
		0: {
			// NOTE: why are we using a Schema v1 to unmarshal configuration with Schema v0?
			// This is possible because the way the unmarshalling works, is that if a field
			// is found in the RawSchema, is then set on the State by looking for a compatible
			// field in the Schema.
			//
			// In other words, fields that are not present in the RawSchema, are simply ignored:
			// the equivalency of RawSchema and Schema is not bidirectional.
			//
			// This works fine _as long as_ there are no renaming or retractions from the Schema.
			// If/when that happens, and a new Schema version is released, then a dedicated
			// schema will be necessary.
			PriorSchema: &schemaV1,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
				var upState privateKeyResourceModel
				res.Diagnostics.Append(req.State.Get(ctx, &upState)...)
				if res.Diagnostics.HasError() {
					return
				}

				// Parse private key from PEM bytes:
				// we do this to generate the missing state from the original private key
				tflog.Debug(ctx, "Parsing private key from PEM")
				prvKey, _, err := parsePrivateKeyPEM([]byte(upState.PrivateKeyPem.ValueString()))
				if err != nil {
					res.Diagnostics.AddError("Unable to parse key from PEM", err.Error())
				}

				// Marshal the Key in OpenSSH PEM, if necessary and supported
				tflog.Debug(ctx, "Marshalling private key to OpenSSH PEM (if supported)")
				if (upState.PrivateKeyOpenSSH.IsNull() || upState.PrivateKeyOpenSSH.ValueString() == "") && prvKeySupportsOpenSSHMarshalling(prvKey) {
					openSSHKeyPemBlock, err := openssh.MarshalPrivateKey(prvKey, "")
					if err != nil {
						res.Diagnostics.AddError("Unable to marshal private key into OpenSSH format", err.Error())
						return
					}

					upState.PrivateKeyOpenSSH = types.StringValue(string(pem.EncodeToMemory(openSSHKeyPemBlock)))
				}

				// Marshal the Key in PKCS#8 PEM
				tflog.Debug(ctx, "Marshalling private key to PKCS#8 PEM")
				prvKeyPKCS8PemBlock, err := prvKeyToPKCS8PEMBlock(prvKey)
				if err != nil {
					res.Diagnostics.AddError("Unable to encode private key to PKCS#8 PEM", err.Error())
					return
				}
				upState.PrivateKeyPKCS8 = types.StringValue(string(pem.EncodeToMemory(prvKeyPKCS8PemBlock)))

				// Upgrading the state
				tflog.Debug(ctx, "Upgrading state")
				res.Diagnostics.Append(res.State.Set(ctx, upState)...)
				if res.Diagnostics.HasError() {
					return
				}

				// Store the rest of the "public key" attributes onto the State
				tflog.Debug(ctx, "Storing private key's public key info into the state")
				res.Diagnostics.Append(setPublicKeyAttributes(ctx, &res.State, prvKey)...)
			},
		},
	}
}

func prvKeyToPKCS8PEMBlock(prvKey interface{}) (*pem.Block, error) {
	keyPKCS8Bytes, err := x509.MarshalPKCS8PrivateKey(prvKey)
	if err != nil {
		return nil, err
	}

	return &pem.Block{
		Type:  PreamblePrivateKeyPKCS8.String(),
		Bytes: keyPKCS8Bytes,
	}, nil
}

func prvKeySupportsOpenSSHMarshalling(prvKey interface{}) bool {
	switch k := prvKey.(type) {
	case *ecdsa.PrivateKey:
		// GOTCHA: `x/crypto/ssh` doesn't handle elliptic curve P-224
		if k.Curve.Params().Name == "P-224" {
			return false
		}
		return true
	default:
		return true
	}
}
