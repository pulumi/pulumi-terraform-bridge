package main

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/providertest/pulumitest/optnewstack"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/grpcutil"
	v6shim "github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/protov6"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestLogReplayProvider(t *testing.T) {
	t.Parallel()
	grpcLogs, err := os.ReadFile("./testdata/TestLogReplayProvider/grpc_log_random.json")
	require.NoError(t, err)

	provPlugin := grpcutil.NewLogReplayProvider("random", "0.0.1", grpcLogs)
	prov := v6shim.New(provPlugin)

	resp, err := prov.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	require.Contains(t, resp.ResourceSchemas, "random_bytes")
	require.Contains(t, resp.ResourceSchemas, "random_pet")
	require.Contains(t, resp.ResourceSchemas, "random_string")

	configVal, err := tfprotov6.NewDynamicValue(
		tftypes.Object{},
		tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{}),
	)
	require.NoError(t, err)
	configResp, err := prov.ValidateProviderConfig(context.Background(), &tfprotov6.ValidateProviderConfigRequest{
		Config: &configVal,
	})

	require.NoError(t, err)
	require.Equal(t, "\x80", string(configResp.PreparedConfig.MsgPack),
		"the config is is msgpack encoded, so we compare the bytes")
}

// TestConfigInDestroy asserts that a nil config is passed to SDKv2 providers on destroy.
//
// This acts as a regression test for https://github.com/pulumi/pulumi-terraform-provider/issues/55.
func TestConfigInDestroy(t *testing.T) {
	t.Parallel()

	logs, err := os.ReadFile("./testdata/TestConfigInDestroy/logs.json")
	require.NoError(t, err)
	p := makeLogReplayProvider(t, "phpipam", "0.0.1", logs)

	ctx := context.Background()
	prov, err := tfbridge.NewProvider(ctx, p, tfbridge.ProviderMetadata{PackageSchema: []byte(`{}`)})
	require.NoError(t, err)

	resp, err := prov.Delete(ctx, plugin.DeleteRequest{
		ID:   "12",
		URN:  "urn:pulumi:prod::space-ipam::phpipam:index/section:Section::section",
		Name: "section",
		Type: "phpipam:index/section:Section",
		Inputs: resource.PropertyMap{
			"name": resource.NewProperty("Section Name"),
		},
		Outputs: resource.PropertyMap{
			reservedkeys.Meta:         resource.NewProperty("{\"private_state\":\"bnVsbA==\"}"),
			"description":             resource.NewProperty(""),
			"displayOrder":            resource.NewProperty(0.0),
			"dnsResolverId":           resource.NewProperty(0.0),
			"editDate":                resource.NewProperty(""),
			"masterSectionId":         resource.NewProperty(0.0),
			"name":                    resource.NewProperty("Section Name"),
			"permissions":             resource.NewProperty(""),
			"phpipamSectionId":        resource.NewProperty("12"),
			"sectionId":               resource.NewProperty(12.0),
			"showSupernetOnly":        resource.NewProperty(false),
			"showVlanInSubnetListing": resource.NewProperty(false),
			"showVrfInSubnetListing":  resource.NewProperty(false),
			"strictMode":              resource.NewProperty(true),
			"subnetOrdering":          resource.NewProperty(""),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, plugin.DeleteResponse{}, resp)
}

type runProvider struct {
	tfprotov6.ProviderServer
	name, version string
}

func (p runProvider) Name() string    { return p.name }
func (p runProvider) Version() string { return p.version }
func (p runProvider) URL() string     { return "url" }
func (p runProvider) Close() error    { return nil }

func makeLogReplayProvider(t *testing.T, name, version string, grpcLogs []byte) info.Provider {
	provPlugin := grpcutil.NewLogReplayProvider(name, version, grpcLogs)
	prov := v6shim.New(provPlugin)
	provider := runProvider{
		ProviderServer: prov,
		name:           name,
		version:        version,
	}

	info, err := providerInfo(context.Background(), provider, parameterize.Value{
		Local: &parameterize.LocalValue{Path: "path"},
	})
	require.NoError(t, err, "failed to read grpc log")

	return info
}

// Asserts that the replayed provider can answer all the engine calls and return the correct output.
func TestLogReplayProviderWithProgram(t *testing.T) {
	t.Parallel()
	grpcLogs, err := os.ReadFile(
		"./testdata/TestLogReplayProvider/grpc_log_random.json")
	if err != nil {
		t.Fatalf("failed to read grpc log: %v", err)
	}

	info := makeLogReplayProvider(t, "random", "0.0.1", grpcLogs)
	program := `
name: proj
runtime: yaml
resources:
  randomPet:
    type: random:Pet
    properties:
        length: 3
outputs:
  petName: ${randomPet.id}`

	pt, err := pulcheck.PulCheck(t, info, program,
		opttest.NewStackOptions(optnewstack.DisableAutoDestroy()),
	)
	require.NoError(t, err)
	res := pt.Up(t)
	require.Equal(t, "curiously-diverse-newt", res.Outputs["petName"].Value)
}
