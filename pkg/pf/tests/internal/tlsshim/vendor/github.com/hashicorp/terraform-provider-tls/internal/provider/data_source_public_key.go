package provider

import (
	"context"
	"crypto"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type publicKeyDataSource struct{}

var _ datasource.DataSource = (*publicKeyDataSource)(nil)

func NewPublicKeyDataSource() datasource.DataSource {
	return &publicKeyDataSource{}
}

func (d *publicKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_public_key"
}

func (d *publicKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Required attributes
			"private_key_pem": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("private_key_pem"),
						path.MatchRoot("private_key_openssh"),
					),
				},
				Description: "The private key (in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format) " +
					"to extract the public key from. " +
					"This is _mutually exclusive_ with `private_key_openssh`. " +
					fmt.Sprintf("Currently-supported algorithms for keys are: `%s`. ", strings.Join(supportedAlgorithmsStr(), "`, `")),
			},
			"private_key_openssh": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("private_key_pem"),
						path.MatchRoot("private_key_openssh"),
					),
				},
				Description: "The private key (in  [OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) format) " +
					"to extract the public key from. " +
					"This is _mutually exclusive_ with `private_key_pem`. " +
					fmt.Sprintf("Currently-supported algorithms for keys are: `%s`. ", strings.Join(supportedAlgorithmsStr(), "`, `")),
			},

			// Computed attributes
			"algorithm": schema.StringAttribute{
				Computed: true,
				Description: "The name of the algorithm used by the given private key. " +
					fmt.Sprintf("Possible values are: `%s`. ", strings.Join(supportedAlgorithmsStr(), "`, `")),
			},
			"public_key_pem": schema.StringAttribute{
				Computed: true,
				Description: "The public key, in [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) format. " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"public_key_openssh": schema.StringAttribute{
				Computed: true,
				Description: "The public key, in  [OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) format. " +
					"This is also known as ['Authorized Keys'](https://www.ssh.com/academy/ssh/authorized_keys/openssh#format-of-the-authorized-keys-file) format. " +
					"This is not populated for `ECDSA` with curve `P224`, as it is [not supported](../../docs#limitations). " +
					"**NOTE**: the [underlying](https://pkg.go.dev/encoding/pem#Encode) " +
					"[libraries](https://pkg.go.dev/golang.org/x/crypto/ssh#MarshalAuthorizedKey) that generate this " +
					"value append a `\\n` at the end of the PEM. " +
					"In case this disrupts your use case, we recommend using " +
					"[`trimspace()`](https://www.terraform.io/language/functions/trimspace).",
			},
			"public_key_fingerprint_md5": schema.StringAttribute{
				Computed: true,
				Description: "The fingerprint of the public key data in OpenSSH MD5 hash format, e.g. `aa:bb:cc:...`. " +
					"Only available if the selected private key format is compatible, as per the rules for " +
					"`public_key_openssh` and [ECDSA P224 limitations](../../docs#limitations).",
			},
			"public_key_fingerprint_sha256": schema.StringAttribute{
				Computed: true,
				Description: "The fingerprint of the public key data in OpenSSH SHA256 hash format, e.g. `SHA256:...`. " +
					"Only available if the selected private key format is compatible, as per the rules for " +
					"`public_key_openssh` and [ECDSA P224 limitations](../../docs#limitations).",
			},
			"id": schema.StringAttribute{
				Computed: true,
				Description: "Unique identifier for this data source: " +
					"hexadecimal representation of the SHA1 checksum of the data source.",
			},
		},
		MarkdownDescription: "Get a public key from a PEM-encoded private key.\n\n" +
			"Use this data source to get the public key from a [PEM (RFC 1421)](https://datatracker.ietf.org/doc/html/rfc1421) " +
			"or [OpenSSH PEM (RFC 4716)](https://datatracker.ietf.org/doc/html/rfc4716) formatted private key, " +
			"for use in other resources.",
	}
}

func (ds *publicKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Debug(ctx, "Reading public key resource")

	var prvKey crypto.PrivateKey
	var algorithm Algorithm
	var err error

	// Given the use of `ExactlyOneOf` in the Schema, we are guaranteed
	// that either `private_key_pem` or `private_key_openssh` will be set.
	var prvKeyArg types.String
	if req.Config.GetAttribute(ctx, path.Root("private_key_pem"), &prvKeyArg); !prvKeyArg.IsNull() && !prvKeyArg.IsUnknown() {
		tflog.Debug(ctx, "Parsing private key from PEM")
		prvKey, algorithm, err = parsePrivateKeyPEM([]byte(prvKeyArg.ValueString()))
	} else if req.Config.GetAttribute(ctx, path.Root("private_key_openssh"), &prvKeyArg); !prvKeyArg.IsNull() && !prvKeyArg.IsUnknown() {
		tflog.Debug(ctx, "Parsing private key from OpenSSH PEM")
		prvKey, algorithm, err = parsePrivateKeyOpenSSHPEM([]byte(prvKeyArg.ValueString()))
	}
	if err != nil {
		res.Diagnostics.AddError("Unable to parse private key", err.Error())
		return
	}

	tflog.Debug(ctx, "Storing private key algorithm info into the state")
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root("algorithm"), &algorithm)...)
	if res.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Storing private key's public key info into the state")
	res.Diagnostics.Append(setPublicKeyAttributes(ctx, &res.State, prvKey)...)
}
