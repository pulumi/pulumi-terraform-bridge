package crosstests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unsafe"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/providerserver"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// RefreshResult captures the outcome of refreshing the same resource through Terraform and Pulumi.
//
// Today this helper focuses on refresh success or failure parity and returns the raw errors so
// regression tests can assert on implementation-specific details when needed.
type RefreshResult struct {
	TFRefreshErr        error
	PulumiRefreshErr    error
	PulumiRefreshResult auto.RefreshResult
}

type refreshOpts struct {
	resourceInfo      *info.Resource
	puConfig          *resource.PropertyMap
	recoverReadPanics bool
	skipParityCheck   bool
}

// A RefreshOption customizes [Refresh].
type RefreshOption func(*refreshOpts)

// RefreshResourceInfo specifies an [info.Resource] to apply to the resource under test.
func RefreshResourceInfo(info info.Resource) RefreshOption {
	return func(o *refreshOpts) { o.resourceInfo = &info }
}

// RefreshPulumiConfig specifies an explicit config value in Pulumi's value space.
func RefreshPulumiConfig(config resource.PropertyMap) RefreshOption {
	return func(o *refreshOpts) { o.puConfig = &config }
}

// RefreshRecoverReadPanics converts Pulumi provider panics during Read into refresh errors.
//
// This is useful for regression tests that need to assert on the refresh failure instead of
// crashing the entire test process.
func RefreshRecoverReadPanics() RefreshOption {
	return func(o *refreshOpts) { o.recoverReadPanics = true }
}

// RefreshSkipParityCheck disables the default success/failure parity assertion.
func RefreshSkipParityCheck() RefreshOption {
	return func(o *refreshOpts) { o.skipParityCheck = true }
}

// Refresh validates refresh behavior for the same resource under Terraform CLI and Pulumi CLI.
//
// It provisions the resource once through each CLI, then runs refresh and compares whether the
// operation succeeded. Callers can opt out of the parity assertion and inspect the returned errors
// directly for regression tests that intentionally capture a mismatch.
func Refresh(t T, resourceUnderTest *schema.Resource, tfConfig cty.Value, options ...RefreshOption) RefreshResult {
	var opts refreshOpts
	for _, f := range options {
		f(&opts)
	}

	var puConfig resource.PropertyMap
	if opts.puConfig != nil {
		puConfig = *opts.puConfig
	} else {
		puConfig = crosstestsimpl.InferPulumiValue(t,
			shimv2.NewSchemaMap(resourceUnderTest.Schema),
			opts.resourceInfo.GetFields(),
			tfConfig,
		)
	}

	tfwd := t.TempDir()
	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, resourceUnderTest)
	tfd.writePlanApply(t, resourceUnderTest.Schema, defRtype, "example", tfConfig, lifecycleArgs{})
	tfErr := tfd.refreshErr(t, resourceUnderTest.Schema, defRtype, "example", tfConfig, lifecycleArgs{})

	bridgedProvider := pulcheck.BridgedProvider(
		t, defProviderShortName,
		&schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: resourceUnderTest}},
		pulcheck.WithResourceInfo(map[string]*info.Resource{defRtype: opts.resourceInfo}),
	)
	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
	}
	yamlProgram := pd.generateYAML(t, puConfig)

	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))
	if opts.recoverReadPanics {
		pt = pulCheckRecoveringReadPanics(t, bridgedProvider, string(yamlProgram))
	}

	pt.Up(t)
	pulumiRes, pulumiErr := pt.CurrentStack().Refresh(pt.Context())
	t.Logf("pulumi refresh stdout:\n%s", pulumiRes.StdOut)
	t.Logf("pulumi refresh stderr:\n%s", pulumiRes.StdErr)

	if !opts.skipParityCheck {
		require.Equalf(t, tfErr == nil, pulumiErr == nil,
			"terraform refresh error = %v, pulumi refresh error = %v\npulumi stdout:\n%s\npulumi stderr:\n%s",
			tfErr, pulumiErr, pulumiRes.StdOut, pulumiRes.StdErr)
	}

	return RefreshResult{
		TFRefreshErr:        tfErr,
		PulumiRefreshErr:    pulumiErr,
		PulumiRefreshResult: pulumiRes,
	}
}

func pulCheckRecoveringReadPanics(
	t T,
	bridgedProvider info.Provider,
	program string,
) *pulumitest.PulumiTest {
	puwd := t.TempDir()
	program = strings.ReplaceAll(program, "\t", "    ")
	err := os.WriteFile(filepath.Join(puwd, "Pulumi.yaml"), []byte(program), 0o600)
	require.NoError(t, err)

	return pulumitest.NewPulumiTest(t, puwd,
		opttest.Env("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			bridgedProvider.Name,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				prov, err := pulcheck.ProviderServerFromInfo(ctx, bridgedProvider)
				if err != nil {
					return 0, err
				}
				prepareProviderForRecoveredReadPanics(t, prov)

				handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
					Init: func(srv *grpc.Server) error {
						pulumirpc.RegisterResourceProviderServer(srv, &recoveringReadPanicsServer{
							ResourceProviderServer: prov,
						})
						return nil
					},
				})
				if err != nil {
					return 0, err
				}
				return providers.Port(handle.Port), nil
			},
		),
	)
}

type recoveringReadPanicsServer struct {
	pulumirpc.ResourceProviderServer
}

func (r *recoveringReadPanicsServer) Read(
	ctx context.Context,
	req *pulumirpc.ReadRequest,
) (resp *pulumirpc.ReadResponse, err error) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			err = fmt.Errorf("recovered provider panic: %v", panicValue)
		}
	}()
	return r.ResourceProviderServer.Read(ctx, req)
}

func prepareProviderForRecoveredReadPanics(t T, server pulumirpc.ResourceProviderServer) {
	t.Helper()

	wrapped, ok := server.(*providerserver.PanicRecoveringProviderServer)
	require.True(t, ok, "expected panic-recovering provider server")

	setPanicRecoveringProviderServerField(t, wrapped, "logger", logging.NewDiscardSink())
	setPanicRecoveringProviderServerField(t, wrapped, "omitStackTraces", true)
}

func setPanicRecoveringProviderServerField(
	t T,
	server *providerserver.PanicRecoveringProviderServer,
	fieldName string,
	value any,
) {
	t.Helper()

	field := reflect.ValueOf(server).Elem().FieldByName(fieldName)
	require.True(t, field.IsValid(), "missing field %q", fieldName)

	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}
