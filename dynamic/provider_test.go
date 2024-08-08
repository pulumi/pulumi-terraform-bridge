package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	helper "github.com/pulumi/pulumi-terraform-bridge/dynamic/internal/testing"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
)

// globalTempDir is a temporary directory scoped to the entire test cycle.
var globalTempDir string

func TestMain(m *testing.M) {
	var err error
	globalTempDir, err = os.MkdirTemp(os.TempDir(), filepath.Base(os.Args[0]))
	contract.AssertNoErrorf(err, "failed to create tmp dir")

	// Run tests
	exitVal := m.Run()

	contract.Assertf(globalTempDir != "", "globalTempDir cannot be empty")
	contract.AssertNoErrorf(os.RemoveAll(globalTempDir), "failed to remove %s", globalTempDir)

	// Exit with exit value from tests
	os.Exit(exitVal)
}

func TestPrimitiveTypes(t *testing.T) {
	t.Parallel()
	skipWindows(t)

	ctx := context.Background()

	grpc := grpcTestServer(ctx, t)

	t.Run("parameterize", assertGRPCCall(grpc.Parameterize, &pulumirpc.ParameterizeRequest{
		Parameters: &pulumirpc.ParameterizeRequest_Args{
			Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
				Args: []string{pfProviderPath(t)},
			},
		},
	}, noParallel))

	inputProps := func() resource.PropertyMap {
		return resource.PropertyMap{
			"attrBoolRequired":            resource.NewProperty(true),
			"attrStringRequired":          resource.NewProperty("s"),
			"attrIntRequired":             resource.NewProperty(64.0),
			"attrNumberRequired":          resource.NewProperty(12.3456),
			"attrStringDefaultOverridden": resource.NewProperty("overridden"),
		}
	}

	inputs := func() *structpb.Struct { return marshal(inputProps()) }

	with := func(base, layer resource.PropertyMap) resource.PropertyMap {
		dst := base.Copy()
		for k, v := range layer {
			dst[k] = v
		}
		return dst
	}

	outputProps := func() resource.PropertyMap {
		return with(inputProps(), resource.PropertyMap{
			"attrStringDefault":  resource.NewProperty("default-value"),
			"attrStringComputed": resource.NewProperty("t"),
			"id":                 resource.NewProperty("output-id"),
		})
	}

	outputs := func() *structpb.Struct { return marshal(outputProps()) }

	urn := string(resource.NewURN(
		"test", "test", "", "pfprovider:index/primitive:Primitive", "prim",
	))

	t.Run("check", assertGRPCCall(grpc.Check, &pulumirpc.CheckRequest{
		Urn:  urn,
		News: inputs(),
	}))

	t.Run("create(preview)", assertGRPCCall(grpc.Create, &pulumirpc.CreateRequest{
		Preview:    true,
		Urn:        urn,
		Properties: inputs(),
	}))

	t.Run("create", assertGRPCCall(grpc.Create, &pulumirpc.CreateRequest{
		Urn:        urn,
		Properties: inputs(),
	}))

	t.Run("diff(none)", assertGRPCCall(grpc.Diff, &pulumirpc.DiffRequest{
		Id:        "example-id-0",
		Urn:       urn,
		Olds:      outputs(),
		News:      inputs(),
		OldInputs: inputs(),
	}))

	t.Run("diff(some)", assertGRPCCall(grpc.Diff, &pulumirpc.DiffRequest{
		Id:  "example-id-1",
		Urn: urn,
		Olds: marshal(resource.PropertyMap{
			"attrBoolComputed":   resource.NewProperty(false),
			"attrBoolRequired":   resource.NewProperty(true),
			"attrIntComputed":    resource.NewProperty(128.0),
			"attrIntRequired":    resource.NewProperty(64.0),
			"attrNumberRequired": resource.NewProperty(12.3456),
			"attrStringComputed": resource.NewProperty("t"),
			"attrStringRequired": resource.NewProperty("s"),
			"id":                 resource.NewProperty("example-id"),
		}),
		News: marshal(resource.PropertyMap{
			"attrBoolRequired":            resource.NewProperty(true),
			"attrStringRequired":          resource.NewProperty("u"),
			"attrIntRequired":             resource.NewProperty(64.0),
			"attrStringDefaultOverridden": resource.NewProperty("overridden"),
		}),
		OldInputs: inputs(),
	}))

	t.Run("diff(all)", assertGRPCCall(grpc.Diff, &pulumirpc.DiffRequest{
		Id:  "example-id-2",
		Urn: urn,
		Olds: marshal(resource.PropertyMap{
			"attrBoolComputed":   resource.NewProperty(false),
			"attrBoolRequired":   resource.NewProperty(true),
			"attrIntComputed":    resource.NewProperty(128.0),
			"attrIntRequired":    resource.NewProperty(64.0),
			"attrNumberRequired": resource.NewProperty(12.3456),
			"attrStringComputed": resource.NewProperty("t"),
			"attrStringRequired": resource.NewProperty("s"),
			"id":                 resource.NewProperty("example-id"),
		}),
		News: marshal(resource.PropertyMap{
			"attrBoolRequired":   resource.NewProperty(false),
			"attrStringRequired": resource.NewProperty("u"),
			"attrIntRequired":    resource.NewProperty(65.0),
			"attrNumberRequired": resource.NewProperty(12.3456789),
		}),
		OldInputs: inputs(),
	}))

	t.Run("delete", assertGRPCCall(grpc.Delete, &pulumirpc.DeleteRequest{
		Id:         "example-id-delete",
		Urn:        urn,
		Properties: outputs(),
	}))

	t.Run("update", assertGRPCCall(grpc.Update, &pulumirpc.UpdateRequest{
		Id:   "example-update-id",
		Urn:  urn,
		Olds: outputs(),
		News: marshal(with(outputProps(), resource.PropertyMap{
			"attrBoolRequired": resource.NewProperty(false),
		})),
	}))

	t.Run("read", assertGRPCCall(grpc.Read, &pulumirpc.ReadRequest{
		Id:         "example-read-id",
		Urn:        urn,
		Properties: outputs(),
	}))

	t.Run("import", assertGRPCCall(grpc.Read, &pulumirpc.ReadRequest{
		Id:  "example-read-id",
		Urn: urn,
	}))
}

func TestConfigure(t *testing.T) {
	t.Parallel()
	skipWindows(t)

	// We test that the provider can be
	// 1. parameterized
	// 2. configured
	// 3. that the configuration is visible to other resources/invokes
	s := grpcTestServer(context.Background(), t)

	assertGRPCCall(s.Parameterize, &pulumirpc.ParameterizeRequest{
		Parameters: &pulumirpc.ParameterizeRequest_Args{
			Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
				Args: []string{pfProviderPath(t)},
			},
		},
	}, noParallel, expect(autogold.Expect(`{
  "name": "pfprovider",
  "version": "0.0.0"
}`)))(t)

	assertGRPCCall(s.Configure, &pulumirpc.ConfigureRequest{
		Args: marshal(resource.PropertyMap{
			"endpoint": resource.NewProperty("my-endpoint"),
		}),
	}, noParallel, expect(autogold.Expect(`{
  "acceptResources": true,
  "supportsPreview": true
}`)))(t)

	assertGRPCCall(s.Invoke, &pulumirpc.InvokeRequest{
		Tok: "pfprovider:index/getConfigEndpoint:getConfigEndpoint",
	}, noParallel, expect(autogold.Expect(`{
  "return": {
    "endpoint": "my-endpoint"
  }
}`)))(t)
}

func TestCheckConfig(t *testing.T) {
	t.Parallel()
	skipWindows(t)

	s := grpcTestServer(context.Background(), t)

	t.Run("parameterize", assertGRPCCall(s.Parameterize, &pulumirpc.ParameterizeRequest{
		Parameters: &pulumirpc.ParameterizeRequest_Args{
			Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
				Args: []string{pfProviderPath(t)},
			},
		},
	}, noParallel, expect(autogold.Expect(`{
  "name": "pfprovider",
  "version": "0.0.0"
}`))))

	t.Run("check-config", assertGRPCCall(s.CheckConfig, &pulumirpc.CheckRequest{
		News: marshal(resource.PropertyMap{
			"endpoint": resource.NewProperty("explicit endpoint"),
		}),
	}, expect(autogold.Expect(`{
  "inputs": {
    "endpoint": "explicit endpoint"
  }
}`))))

	// TODO: This should error
	t.Run("check-config (invalid)", assertGRPCCall(s.CheckConfig, &pulumirpc.CheckRequest{
		News: marshal(resource.PropertyMap{
			"endpoint": resource.NewProperty(123.456),
		}),
	}, expect(autogold.Expect(`{
  "inputs": {
    "endpoint": "123.456"
  }
}`))))

	// Check that we correctly handle JSON encoded config values sent by Pulumi.
	//
	// This will become unnecessary when https://github.com/pulumi/pulumi/pull/15032
	// merges.
	t.Run("json-encoded-nested-config", assertGRPCCall(s.CheckConfig, &pulumirpc.CheckRequest{
		News: marshal(resource.PropertyMap{
			"nested": resource.NewProperty(`{"field1": "true", "field2": false}`),
		}),
	}, expect(autogold.Expect(`{
  "inputs": {
    "nested": "{\"field1\":\"true\",\"field2\":false}"
  }
}`))))
}

type assertGRPCCallOptions struct {
	noParallel bool
	expect     autogold.Value
}

func noParallel(o *assertGRPCCallOptions) { o.noParallel = true }

func expect(v autogold.Value) assertGRPCCallOption {
	return func(o *assertGRPCCallOptions) {
		o.expect = v
	}
}

type assertGRPCCallOption func(*assertGRPCCallOptions)

// assertGRPCCall makes a gRPC call and then asserts on the result using [assertGRPC].
func assertGRPCCall[T any, R proto.Message](
	method func(context.Context, T) (R, error), req T,
	opts ...assertGRPCCallOption,
) func(*testing.T) {
	var o assertGRPCCallOptions
	for _, opt := range opts {
		opt(&o)
	}
	return func(t *testing.T) {
		t.Helper()
		if !o.noParallel {
			t.Parallel()
		}
		resp, err := method(context.Background(), req)
		require.NoError(t, err)
		assertGRPC(t, resp, o.expect)
	}
}

// assertGRPC uses autogold to check/save msg.
func assertGRPC(t *testing.T, msg proto.Message, v autogold.Value) {
	t.Helper()
	j, err := protojson.MarshalOptions{
		Multiline: true,
	}.Marshal(msg)
	require.NoError(t, err)

	// We now re-marshal and re-un-marshal to get deterministic output from
	// protojson. protojson inserts random spaces to ensure that output is
	// non-deterministic here:
	// https://github.com/protocolbuffers/protobuf-go/blob/d4621760eaa24af1d915dd112919dbb53f94db01/internal/encoding/json/encode.go#L239-L243
	//
	//nolint:lll
	var m map[string]any
	require.NoError(t, json.Unmarshal(j, &m))
	j, err = json.MarshalIndent(m, "", "  ")
	require.NoError(t, err)
	if v == nil {
		autogold.ExpectFile(t, autogold.Raw(string(j)))
	} else {
		v.Equal(t, string(j))
	}
}

// pfProviderPath returns the path the the PF provider binary for use in testing.
//
// It builds the binary running "go build" once per session.
var pfProviderPath = func() func(t *testing.T) string {
	mkBin := sync.OnceValues(func() (string, error) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		out := filepath.Join(globalTempDir, "terraform-provider-pfprovider")
		cmd := exec.Command("go", "build", "-o", out, "github.com/pulumi/pulumi-terraform-bridge/dynamic/tests/pfprovider")
		cmd.Dir = filepath.Join(wd, "test", "pfprovider")
		stdoutput, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to build provider: %w:\n%s", err, string(stdoutput))
		}
		return out, nil
	})

	return func(t *testing.T) string {
		t.Helper()
		path, err := mkBin()
		require.NoErrorf(t, err, "failed find provider path")
		return path
	}
}()

func grpcTestServer(ctx context.Context, t *testing.T) pulumirpc.ResourceProviderServer {
	defaultInfo, metadata, close := initialSetup()
	t.Cleanup(func() { assert.NoError(t, close()) })
	s, err := pfbridge.NewProviderServer(ctx, nil, defaultInfo, metadata)
	require.NoError(t, err)
	return s
}

func skipWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	t.Skipf("autogold does not play nice with windows newlines")
}

func TestSchemaGeneration(t *testing.T) {
	skipWindows(t)

	t.Run("unparameterized", func(t *testing.T) {
		helper.Integration(t)
		ctx := context.Background()

		schema, err := grpcTestServer(ctx, t).
			GetSchema(ctx, &pulumirpc.GetSchemaRequest{})

		require.NoError(t, err)
		var fmtSchema bytes.Buffer
		require.NoError(t, json.Indent(&fmtSchema, []byte(schema.Schema), "", "    "))
		autogold.ExpectFile(t, autogold.Raw(fmtSchema.String()))
	})

	testSchema := func(name, version string) {
		t.Run(strings.Join([]string{name, version}, "-"), func(t *testing.T) {
			helper.Integration(t)
			ctx := context.Background()

			server := grpcTestServer(ctx, t)

			result, err := server.Parameterize(ctx, &pulumirpc.ParameterizeRequest{
				Parameters: &pulumirpc.ParameterizeRequest_Args{
					Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
						Args: []string{name, version},
					},
				},
			})
			require.NoError(t, err)

			assert.Equal(t, version, result.Version)

			schema, err := server.GetSchema(ctx, &pulumirpc.GetSchemaRequest{
				SubpackageName:    result.Name,
				SubpackageVersion: result.Version,
			})

			require.NoError(t, err)
			var fmtSchema bytes.Buffer
			require.NoError(t, json.Indent(&fmtSchema, []byte(schema.Schema), "", "    "))
			autogold.ExpectFile(t, autogold.Raw(fmtSchema.String()))
		})
	}

	testSchema("hashicorp/random", "3.3.0")
	testSchema("Azure/alz", "0.11.1")
	testSchema("Backblaze/b2", "0.8.9")
}

func TestRandomCreate(t *testing.T) {
	ctx := context.Background()
	server := grpcTestServer(ctx, t)
	parameterizeResp, err := server.Parameterize(ctx, &pulumirpc.ParameterizeRequest{
		Parameters: &pulumirpc.ParameterizeRequest_Args{
			Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
				Args: []string{"hashicorp/random", "=3.3.0"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &pulumirpc.ParameterizeResponse{
		Name:    "random",
		Version: "3.3.0",
	}, parameterizeResp)

	t.Run("preview", func(t *testing.T) {
		resp, err := server.Create(ctx, &pulumirpc.CreateRequest{
			Urn:     string(resource.NewURN("dev", "test", "", "random:index/string:String", "name")),
			Preview: true,
			Properties: must(plugin.MarshalProperties(resource.PropertyMap{
				"length": resource.NewProperty(6.0),
			}, plugin.MarshalOptions{})),
		})
		require.NoError(t, err)

		// We do not use [assertGRPC] here because we want to use a testing method
		// that works on windows in at least one test.
		var actual map[string]any
		require.NoError(t, json.Unmarshal(must(protojson.MarshalOptions{}.Marshal(resp)), &actual))
		autogold.Expect(map[string]interface{}{"properties": map[string]interface{}{
			"id": "04da6b54-80e4-46f7-96ec-b56ff0331ba9", "length": 6,
			"lower":      true,
			"minLower":   0,
			"minNumeric": 0,
			"minSpecial": 0,
			"minUpper":   0,
			"number":     true,
			"numeric":    true,
			"result":     "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
			"special":    true,
			"upper":      true,
		}}).Equal(t, actual)
	})

	t.Run("up", func(t *testing.T) {
		for _, i := range []int{3, 8, 12} {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				createResp, err := server.Create(ctx, &pulumirpc.CreateRequest{
					Urn: string(resource.NewURN("dev", "test", "", "random:index/string:String", "name")),
					Properties: must(plugin.MarshalProperties(resource.PropertyMap{
						"length": resource.NewProperty(float64(i)),
					}, plugin.MarshalOptions{})),
				})
				require.NoError(t, err)
				assert.Len(t, createResp.Id, i)
			})
		}
	})
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func marshal(m resource.PropertyMap) *structpb.Struct {
	return must(plugin.MarshalProperties(m, plugin.MarshalOptions{}))
}
