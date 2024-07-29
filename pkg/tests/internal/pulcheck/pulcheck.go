package pulcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	pulumidiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"gotest.tools/assert"
)

func propNeedsUpdate(prop *schema.Schema) bool {
	if prop.Computed && !prop.Optional {
		return false
	}
	if prop.ForceNew {
		return false
	}
	return true
}

func resourceNeedsUpdate(res *schema.Resource) bool {
	// If any of the properties need an update, then the resource needs an update.
	for _, s := range res.Schema {
		if propNeedsUpdate(s) {
			return true
		}
	}
	return false
}

// This is an experimental API.
func EnsureProviderValid(t T, tfp *schema.Provider) {
	for _, r := range tfp.ResourcesMap {
		if r.ReadContext == nil {
			r.ReadContext = func(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
				return nil
			}
		}
		if r.DeleteContext == nil {
			r.DeleteContext = func(
				ctx context.Context, rd *schema.ResourceData, i interface{},
			) diag.Diagnostics {
				return diag.Diagnostics{}
			}
		}

		if r.CreateContext == nil {
			r.CreateContext = func(
				ctx context.Context, rd *schema.ResourceData, i interface{},
			) diag.Diagnostics {
				rd.SetId("newid")
				return diag.Diagnostics{}
			}
		}

		if resourceNeedsUpdate(r) && r.UpdateContext == nil {
			r.UpdateContext = func(
				ctx context.Context, rd *schema.ResourceData, i interface{},
			) diag.Diagnostics {
				return diag.Diagnostics{}
			}
		}
	}
	require.NoError(t, tfp.InternalValidate())
}

// This is an experimental API.
func StartPulumiProvider(ctx context.Context, name, version string, providerInfo tfbridge.ProviderInfo) (*rpcutil.ServeHandle, error) {
	sink := pulumidiag.DefaultSink(io.Discard, io.Discard, pulumidiag.FormatOptions{
		Color: colors.Never,
	})

	schema, err := tfgen.GenerateSchema(providerInfo, sink)
	if err != nil {
		return nil, fmt.Errorf("tfgen.GenerateSchema failed: %w", err)
	}

	schemaBytes, err := json.MarshalIndent(schema, "", " ")
	if err != nil {
		return nil, fmt.Errorf("json.MarshalIndent(schema, ..) failed: %w", err)
	}

	prov := tfbridge.NewProvider(ctx, nil, name, version, providerInfo.P, providerInfo, schemaBytes)

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rpcutil.ServeWithOptions failed: %w", err)
	}

	return &handle, nil
}

// This is an experimental API.
type T interface {
	Logf(string, ...any)
	TempDir() string
	Skip(...any)
	require.TestingT
	assert.TestingT
	pulumitest.PT
}

type bridgedProviderOpts struct {
	DisablePlanResourceChange bool
	ProviderSchema            map[string]*schema.Schema
	ConfigureContextFunc      schema.ConfigureContextFunc
}

// BridgedProviderOpts
type BridgedProviderOpt func(*bridgedProviderOpts)

// WithPlanResourceChange
func DisablePlanResourceChange() BridgedProviderOpt {
	return func(o *bridgedProviderOpts) {
		o.DisablePlanResourceChange = true
	}
}

// This is an experimental API.
func BridgedProvider(t T, providerName string, tfp *schema.Provider, opts ...BridgedProviderOpt) info.Provider {
	options := &bridgedProviderOpts{}
	for _, opt := range opts {
		opt(options)
	}

	EnsureProviderValid(t, tfp)

	shimProvider := shimv2.NewProvider(tfp, shimv2.WithPlanResourceChange(
		func(tfResourceType string) bool { return !options.DisablePlanResourceChange },
	))

	provider := tfbridge.ProviderInfo{
		P:                              shimProvider,
		Name:                           providerName,
		Version:                        "0.0.1",
		MetadataInfo:                   &tfbridge.MetadataInfo{},
		EnableZeroDefaultSchemaVersion: true,
	}
	makeToken := func(module, name string) (string, error) {
		return tokens.MakeStandard(providerName)(module, name)
	}
	provider.MustComputeTokens(tokens.SingleModule(providerName, "index", makeToken))

	return provider
}

func skipUnlessLinux(t T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}

// This is an experimental API.
func PulCheck(t T, bridgedProvider info.Provider, program string) *pulumitest.PulumiTest {
	skipUnlessLinux(t)
	puwd := t.TempDir()
	p := filepath.Join(puwd, "Pulumi.yaml")

	program = strings.ReplaceAll(program, "\t", "    ")
	err := os.WriteFile(p, []byte(program), 0o600)
	require.NoError(t, err)

	opts := []opttest.Option{
		opttest.Env("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			bridgedProvider.Name,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := StartPulumiProvider(ctx, bridgedProvider.Name, bridgedProvider.Version, bridgedProvider)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	}

	return pulumitest.NewPulumiTest(t, puwd, opts...)
}
