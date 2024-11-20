package tfbridgetests

import (
	"fmt"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"
)

func TestSecretBasic(t *testing.T) {
	t.Parallel()
	provBuilder := providerbuilder.NewProvider(
		providerbuilder.NewProviderArgs{
			AllResources: []providerbuilder.Resource{
				providerbuilder.NewResource(providerbuilder.NewResourceArgs{
					ResourceSchema: rschema.Schema{
						Attributes: map[string]rschema.Attribute{
							"s": rschema.StringAttribute{Optional: true},
						},
					},
				}),
			},
		})

	prov := bridgedProvider(provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            s:
                fn::secret:
                    %s`

	pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "hello"))
	require.NoError(t, err)
	pt.Up(t)

	pt.WritePulumiYaml(t, fmt.Sprintf(program, "value2"))
	res := pt.Preview(t, optpreview.Diff())
	autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        s: [secret]
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
}

func TestSecretSet(t *testing.T) {
	t.Parallel()

	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			providerbuilder.NewResource(providerbuilder.NewResourceArgs{
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"keys": rschema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			}),
		},
	})

	prov := bridgedProvider(provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            keys: %s
`

	t.Run("secret collection", func(t *testing.T) {
		t.Parallel()
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "{fn::secret: [value1, value2]}"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "{fn::secret: [value1, value3]}"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        keys: [secret]
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})

	t.Run("secret collection element", func(t *testing.T) {
		t.Parallel()
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "[{fn::secret: value1}, value2]"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "[{fn::secret: value3}, value2]"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        keys: [secret]
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})

	t.Run("secret collection element secret element unchanged", func(t *testing.T) {
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "[{fn::secret: value1}, value2]"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "[{fn::secret: value1}, value3]"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        keys: [secret]
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})
}

func TestSecretObjectBlock(t *testing.T) {
	t.Parallel()

	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			providerbuilder.NewResource(providerbuilder.NewResourceArgs{
				ResourceSchema: rschema.Schema{
					Blocks: map[string]rschema.Block{
						"key": rschema.SingleNestedBlock{
							Attributes: map[string]rschema.Attribute{
								"prop1": rschema.StringAttribute{Optional: true},
								"prop2": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			}),
		},
	})

	prov := bridgedProvider(provBuilder)

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            key: %s`

	t.Run("secret object block", func(t *testing.T) {
		t.Parallel()
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "{fn::secret: {prop1: value1, prop2: value2}}"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "{fn::secret: {prop1: value3, prop2: value2}}"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        key: [secret]
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})

	t.Run("secret object block element", func(t *testing.T) {
		t.Parallel()
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "{prop1: {fn::secret: value1}, prop2: value2}"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "{prop1: {fn::secret: value3}, prop2: value2}"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        key: {
            prop1: [secret]
            prop2: "value2"
        }
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})

	t.Run("secret object block element secret element unchanged", func(t *testing.T) {
		t.Parallel()
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "{prop1: {fn::secret: value1}, prop2: value2}"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "{prop1: {fn::secret: value1}, prop2: value3}"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
      ~ key: {
            prop1: [secret]
          ~ prop2: "value2" => "value3"
        }
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})
}

func TestSecretPulumiSchema(t *testing.T) {
	t.Parallel()

	provBuilder := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []providerbuilder.Resource{
			providerbuilder.NewResource(providerbuilder.NewResourceArgs{
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"s": rschema.StringAttribute{Optional: true},
					},
				},
			}),
		},
	})

	prov := bridgedProvider(provBuilder)

	prov.Resources["testprovider_test"].Fields = map[string]*info.Schema{
		"s": {Secret: tfbridge0.True()},
	}

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: testprovider:index:Test
        properties:
            s: %s`

	t.Run("secret string attribute", func(t *testing.T) {
		t.Parallel()
		pt, err := pulcheck.PulCheck(t, prov, fmt.Sprintf(program, "value1"))
		require.NoError(t, err)
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, "value2"))
		res := pt.Preview(t, optpreview.Diff())
		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::test::testprovider:index/test:Test::mainRes]
        s: [secret]
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, res.StdOut)
	})
}
