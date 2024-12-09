package pulcheck

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

type T = crosstestsimpl.T

func testSink(t T) diag.Sink {
	var stdout, stderr bytes.Buffer

	testSink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	t.Cleanup(func() {
		if strings.TrimSpace(stdout.String()) != "" {
			t.Logf("%s\n", stdout.String())
		}
		if strings.TrimSpace(stderr.String()) != "" {
			t.Logf("%s\n", stderr.String())
		}
	})

	return testSink
}

func genMetadata(t T, info tfbridge0.ProviderInfo) (tfbridge.ProviderMetadata, error) {
	generated, err := tfgen.GenerateSchema(context.Background(), tfgen.GenerateSchemaOptions{
		ProviderInfo:    info,
		DiagnosticsSink: testSink(t),
		XInMemoryDocs:   true,
	})
	if err != nil {
		return tfbridge.ProviderMetadata{}, err
	}
	return generated.ProviderMetadata, nil
}

func newProviderServer(t T, info tfbridge0.ProviderInfo) (pulumirpc.ResourceProviderServer, error) {
	ctx := context.Background()
	meta, err := genMetadata(t, info)
	if err != nil {
		return nil, err
	}
	srv, err := tfbridge.NewProviderServer(ctx, nil, info, meta)
	require.NoError(t, err)
	return srv, nil
}

func startPulumiProvider(t T, prov pulumirpc.ResourceProviderServer) *rpcutil.ServeHandle {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	require.NoErrorf(t, err, "rpcutil.ServeWithOptions failed")
	return &handle
}

func skipUnlessLinux(t T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		// TODO[pulumi/pulumi-terraform-bridge#2221]
		t.Skip("Skipping on non-Linux platforms")
	}
}

// PulCheck creates a new Pulumi test from a bridged provider and a program.
func PulCheck(t T, bridgedProvider info.Provider, program string, opts ...opttest.Option) (*pulumitest.PulumiTest, error) {
	skipUnlessLinux(t)
	t.Helper()
	puwd := t.TempDir()
	p := filepath.Join(puwd, "Pulumi.yaml")

	err := os.WriteFile(p, []byte(program), 0o600)
	require.NoError(t, err)

	prov, err := newProviderServer(t, bridgedProvider)
	if err != nil {
		return nil, err
	}

	defaultOpts := []opttest.Option{
		opttest.Env("DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			bridgedProvider.Name,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle := startPulumiProvider(t, prov)
				return providers.Port(handle.Port), nil
			},
		),
	}

	defaultOpts = append(defaultOpts, opts...)

	return pulumitest.NewPulumiTest(t, puwd, defaultOpts...), nil
}
