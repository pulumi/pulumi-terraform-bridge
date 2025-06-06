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
	pulumidiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"gotest.tools/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
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

// resourceNeedsUpdate returns true if the TF SDK schema validation would consider the
// resource to need an update method.
func resourceNeedsUpdate(res *schema.Resource) bool {
	// If any of the properties need an update, then the resource needs an update.
	for _, s := range res.Schema {
		if propNeedsUpdate(s) {
			return true
		}
	}
	return false
}

func copyMap[K comparable, V any](m map[K]V, cp func(V) V) map[K]V {
	dst := make(map[K]V, len(m))
	for k, v := range m {
		dst[k] = cp(v)
	}
	return dst
}

// WithValidProvider returns a copy of tfp, with all required fields filled with default
// values.
//
// This is an experimental API.
func WithValidProvider(t T, tfp *schema.Provider) *schema.Provider {
	if tfp == nil {
		return nil
	}

	// Copy tfp as deep as we will mutate.
	{
		dst := *tfp // memcopy
		dst.ResourcesMap = copyMap(tfp.ResourcesMap, func(v *schema.Resource) *schema.Resource {
			cp := *v // memcopy
			cp.Schema = copyMap(cp.Schema, func(s *schema.Schema) *schema.Schema {
				cp := *s
				return &cp
			})
			return &cp
		})
		tfp = &dst
	}

	// Now ensure that tfp is valid

	for _, r := range tfp.ResourcesMap {
		// If the resource will be flagged as not needing an Update method by the TF SDK schema
		// validation then add a no-op update_prop property that will instead force the resource
		// to need an Update method.
		// This is necessary to work around a bug in the TF SDK schema validation where some resources which
		// do need an Update method will instead be flagged as not needing an Update method.
		// See https://github.com/pulumi/pulumi-terraform-bridge/pull/2723#issuecomment-2541518646
		if !resourceNeedsUpdate(r) {
			r.Schema["update_prop"] = &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			}
		}

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

		// Because of the no-op update_prop property added above, all resources will now
		// need an Update method.
		if r.UpdateContext == nil {
			r.UpdateContext = func(
				ctx context.Context, rd *schema.ResourceData, i interface{},
			) diag.Diagnostics {
				return diag.Diagnostics{}
			}
		}
	}
	require.NoError(t, tfp.InternalValidate())

	return tfp
}

func ProviderServerFromInfo(
	ctx context.Context, providerInfo tfbridge.ProviderInfo,
) (pulumirpc.ResourceProviderServer, error) {
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

	return tfbridge.NewProvider(
		ctx, nil, providerInfo.Name, providerInfo.Version, providerInfo.P, providerInfo, schemaBytes), nil
}

// This is an experimental API.
func StartPulumiProvider(ctx context.Context, providerInfo tfbridge.ProviderInfo) (*rpcutil.ServeHandle, error) {
	prov, err := ProviderServerFromInfo(ctx, providerInfo)
	if err != nil {
		return nil, fmt.Errorf("ProviderServerFromInfo failed: %w", err)
	}

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
	StateEdit    shimv2.PlanStateEditFunc
	resourceInfo map[string]*info.Resource
	configInfo   map[string]*info.Schema
}

// BridgedProviderOpts
type BridgedProviderOpt func(*bridgedProviderOpts)

func WithStateEdit(f shimv2.PlanStateEditFunc) BridgedProviderOpt {
	return func(o *bridgedProviderOpts) {
		o.StateEdit = f
	}
}

// WithResourceInfo allows the user to set the info.Provider.Resources field within a
// [BridgedProvider].
//
// This is an experimental API.
func WithResourceInfo(info map[string]*info.Resource) BridgedProviderOpt {
	return func(o *bridgedProviderOpts) { o.resourceInfo = info }
}

// WithResourceInfo allows the user to set the info.Provider.Config field within a
// [BridgedProvider].
//
// This is an experimental API.
func WithConfigInfo(info map[string]*info.Schema) BridgedProviderOpt {
	return func(o *bridgedProviderOpts) { o.configInfo = info }
}

// This is an experimental API.
func BridgedProvider(t T, providerName string, tfp *schema.Provider, opts ...BridgedProviderOpt) info.Provider {
	var options bridgedProviderOpts
	for _, opt := range opts {
		opt(&options)
	}

	tfp = WithValidProvider(t, tfp)

	shimProvider := shimv2.NewProvider(tfp,
		shimv2.WithPlanStateEdit(options.StateEdit),
	)

	provider := tfbridge.ProviderInfo{
		P:                              shimProvider,
		Name:                           providerName,
		Version:                        "0.0.1",
		MetadataInfo:                   &tfbridge.MetadataInfo{},
		EnableZeroDefaultSchemaVersion: true,
		Resources:                      options.resourceInfo,
		Config:                         options.configInfo,
	}
	provider.MustComputeTokens(tokens.SingleModule(providerName,
		"index", tokens.MakeStandard(providerName)))

	return provider
}

func skipUnlessLinux(t T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}

// This is an experimental API.
func PulCheck(t T, bridgedProvider info.Provider, program string, opts ...opttest.Option) *pulumitest.PulumiTest {
	skipUnlessLinux(t)
	puwd := t.TempDir()
	p := filepath.Join(puwd, "Pulumi.yaml")

	program = strings.ReplaceAll(program, "\t", "    ")
	err := os.WriteFile(p, []byte(program), 0o600)
	require.NoError(t, err)

	defaultOpts := []opttest.Option{
		opttest.Env("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			bridgedProvider.Name,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := StartPulumiProvider(ctx, bridgedProvider)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	}

	defaultOpts = append(defaultOpts, opts...)
	return pulumitest.NewPulumiTest(t, puwd, defaultOpts...)
}
