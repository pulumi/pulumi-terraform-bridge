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
	helper "github.com/pulumi/pulumi-terraform-bridge/dynamic/internal/testing"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	_, err := grpc.Parameterize(ctx, &pulumirpc.ParameterizeRequest{
		Parameters: &pulumirpc.ParameterizeRequest_Args{
			Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
				Args: []string{pfProviderPath(t)},
			},
		},
	})
	require.NoError(t, err)

	inputs := must(plugin.MarshalProperties(resource.PropertyMap{
		"attrBoolRequired":   resource.NewProperty(true),
		"attrStringRequired": resource.NewProperty("s"),
		"attrIntRequired":    resource.NewProperty(64.0),
	}, plugin.MarshalOptions{}))

	t.Run("check", func(t *testing.T) {
		resp, err := grpc.Check(ctx, &pulumirpc.CheckRequest{
			Urn: string(resource.NewURN(
				"test", "test", "", "pfprovider:index/primitive:Primitive", "prim",
			)),
			News: inputs,
		})
		require.NoError(t, err)
		assertGRPC(t, resp)
	})

	t.Run("create(preview)", func(t *testing.T) {
		resp, err := grpc.Create(ctx, &pulumirpc.CreateRequest{
			Preview: true,
			Urn: string(resource.NewURN(
				"test", "test", "", "pfprovider:index/primitive:Primitive", "prim",
			)),
			Properties: inputs,
		})
		require.NoError(t, err)
		assertGRPC(t, resp)
	})

	t.Run("create", func(t *testing.T) {
		resp, err := grpc.Create(ctx, &pulumirpc.CreateRequest{
			Urn: string(resource.NewURN(
				"test", "test", "", "pfprovider:index/primitive:Primitive", "prim",
			)),
			Properties: inputs,
		})
		require.NoError(t, err)
		assertGRPC(t, resp)
	})
}

// assertGRPC uses autogold to check/save msg.
func assertGRPC(t *testing.T, msg proto.Message) {
	t.Helper()
	autogold.ExpectFile(t, autogold.Raw(must(protojson.MarshalOptions{
		Multiline: true,
	}.Marshal(msg))))
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
		return out, cmd.Run()
	})

	return func(t *testing.T) string {
		t.Helper()
		path, err := mkBin()
		require.NoError(t, err)
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
